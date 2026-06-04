//go:build integration_test

package featureassignment_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// reconcileFunc abstracts the old and new reconciler behind one call.
type reconcileFunc func(ctx context.Context) error

// sqlPublisher records published messages and simulates naisd status updates
// via raw SQL so it works independently of the deployment context loader.
type sqlPublisher struct {
	pool *pgxpool.Pool
	msg  []message.DeployInstruction
}

func (p *sqlPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)
	status := model.RolloutStatusDeployed
	if strings.HasSuffix(msg.Name, "-pending") {
		status = model.RolloutStatusPending
	}
	_, err := p.pool.Exec(ctx, `UPDATE deploy_instructions SET status = $1 WHERE id = $2`, status.String(), msg.ID)
	return err
}

func (p *sqlPublisher) Stop() {}

type reconcileDB struct {
	Db
}

func (d *reconcileDB) queryDeployedInstructions(ctx context.Context, t *testing.T) []string {
	t.Helper()
	rows, err := d.pool.Query(ctx, `
		SELECT t.name || ':' || e.name || ':' || di.feature_name || ':' || di.feature_version
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed'
		ORDER BY 1
	`)
	if err != nil {
		t.Fatalf("query deployed instructions: %v", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		result = append(result, s)
	}
	return result
}

func (d *reconcileDB) queryDeployedFeatures(ctx context.Context, t *testing.T, featureName string) []string {
	t.Helper()
	rows, err := d.pool.Query(ctx, `
		SELECT t.name || ':' || e.name
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed' AND di.feature_name = $1
		ORDER BY 1
	`, featureName)
	if err != nil {
		t.Fatalf("query deployed features: %v", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		result = append(result, s)
	}
	return result
}

func (d *reconcileDB) countInstructions(ctx context.Context, t *testing.T, featureName, version string) int {
	t.Helper()
	var count int
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM deploy_instructions
		WHERE feature_name = $1 AND feature_version = $2
	`, featureName, version).Scan(&count)
	if err != nil {
		t.Fatalf("count instructions: %v", err)
	}
	return count
}

// setupReconcileTest creates a fresh DB, wires both reconcilers, and returns
// everything needed to run a test case against either implementation.
func setupReconcileTest(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string) (
	newCtx context.Context,
	db *reconcileDB,
	pub *sqlPublisher,
	newReconcile reconcileFunc,
	seeder *featureassignmenttest.Seeder,
	rec *reconciler.Reconciler,
	lastResult **reconciler.DesiredState,
) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if err := container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	pub = &sqlPublisher{pool: pool}
	seeder = featureassignmenttest.NewSeeder()

	// Wire old reconciler via context loader.
	loadContext, err := contextloader.NewLoaderFunc(pool, logger)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}
	newCtx = loadContext(ctx)

	// Wire new reconciler.
	newPub := func(topicID string, log *slog.Logger) reconciler.Publisher { return pub }
	dispatcher, err := reconciler.NewPubSubDispatcher(pool, newPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create result writer: %v", err)
	}
	rec, err = reconciler.New(pool, meter, logger)
	if err != nil {
		t.Fatalf("failed to create reconciler: %v", err)
	}
	var lr *reconciler.DesiredState
	lastResult = &lr
	newReconcile = func(ctx context.Context) error {
		result, err := rec.ComputeDesiredState(ctx)
		if err != nil {
			return err
		}
		lr = result
		return dispatcher.Dispatch(ctx, result.Decisions)
	}

	db = &reconcileDB{Db{t: t, pool: pool}}
	return
}

type featureInput struct {
	name, version string
	dependencies  []string
	target        environment.Labels
}

// forEachReconciler runs fn once for the old reconciler and once for the new,
// as subtests named "old" and "new". Both start from the same seeded state.
// Each reconcile call is timed and the duration is logged to the test output.
// TODO: rewrite since we now only have one reconciler
func forEachReconciler(
	t *testing.T,
	ctx context.Context,
	container *postgres.PostgresContainer,
	dsn string,
	fn func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder),
) {
	t.Helper()
	for _, impl := range []struct {
		name string
		pick func(old, new reconcileFunc) reconcileFunc
	}{
		{"new", func(_, new reconcileFunc) reconcileFunc { return new }},
	} {
		t.Run(impl.name, func(t *testing.T) {
			ctx, db, pub, newR, seeder, _, lastResultPtr := setupReconcileTest(ctx, t, container, dsn)
			featureassignment.ChartDownloader = seeder.ChartDownloader()

			call := 0
			raw := newR
			timed := func(ctx context.Context) error {
				call++
				start := time.Now()
				err := raw(ctx)
				elapsed := time.Since(start)
				t.Logf("reconcile #%d took %s", call, elapsed)
				if *lastResultPtr != nil {
					lr := *lastResultPtr
					ioDur := elapsed - lr.FetchDur - lr.ComputeDur
					t.Logf("  phases: fetch=%s compute=%s io=%s", lr.FetchDur, lr.ComputeDur, ioDur)
				}
				return err
			}

			fn(t, ctx, db, pub, timed, seeder)
		})
	}
}

// ---------- Test cases ----------

func TestReconcile(t *testing.T) {
	t.Skip("skipping for now")
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	envsToCreate := map[string]environment.Labels{
		"test-partner:dev":  {},
		"test-partner:prod": {"featuretoggle": "enabled"},
		"nav:dev":           {"aiven": "enabled"},
		"nav:management":    {"kind": "management"},
	}

	tt := []struct {
		name                string
		deploymentsToCreate []featureInput
		reconcileResults    [][]string
	}{
		{
			name: "install most specific and latest features",
			deploymentsToCreate: []featureInput{
				{name: "aivenator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "aivenator", version: "2.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "aivenator", version: "1.1.0", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
				{name: "aivenator", version: "1.1.1", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
				{name: "aivenator", version: "3.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "naiserator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "unleash", version: "1.0.0", target: environment.Labels{"featuretoggle": "enabled"}},
				{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
				{name: "v13s", version: "1.0.0", target: environment.Labels{"kind": "management"}},
			},
			reconcileResults: [][]string{
				{
					"nav:dev:aivenator:1.1.1",
					"nav:dev:naiserator:1.0.0",
					"nav:dev:unleash:2.0.0",
					"nav:management:v13s:1.0.0",
					"test-partner:dev:unleash:2.0.0",
					"test-partner:prod:unleash:2.0.0",
				},
			},
		},
		{
			name: "install features with dependencies",
			deploymentsToCreate: []featureInput{
				{
					name:         "monitoring",
					version:      "v1",
					dependencies: []string{"monitoring-crds"},
					target:       environment.Labels{"tenant": "nav"},
				},
				{
					name:    "monitoring-crds",
					version: "v1",
					target:  environment.Labels{"tenant": "nav"},
				},
			},
			reconcileResults: [][]string{
				{
					"nav:dev:monitoring-crds:v1",
					"nav:management:monitoring-crds:v1",
				},
				{
					"nav:dev:monitoring-crds:v1",
					"nav:management:monitoring-crds:v1",
					"nav:dev:monitoring:v1",
					"nav:management:monitoring:v1",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
				db.createTenantsAndEnvironments(ctx, envsToCreate)
				for _, input := range tc.deploymentsToCreate {
					seeder.AddAssignment(input.name, input.version, input.target, input.dependencies...)
				}
				if _, err := seeder.Seed(ctx); err != nil {
					t.Fatalf("seeding: %v", err)
				}

				for _, expected := range tc.reconcileResults {
					if err := reconcile(ctx); err != nil {
						t.Fatalf("reconcile: %v", err)
					}

					instructions := db.queryDeployedInstructions(ctx, t)
					sorted := make([]string, len(expected))
					copy(sorted, expected)
					sort.Strings(sorted)

					if !slices.Equal(sorted, instructions) {
						t.Errorf("instructions mismatch:\ngot:  %v\nwant: %v", instructions, sorted)
					}
					if len(pub.msg) != len(expected) {
						t.Fatalf("pub.msg len = %d, want %d", len(pub.msg), len(expected))
					}

					for _, exp := range expected {
						found := false
						for _, msg := range pub.msg {
							parts := strings.Split(exp, ":")
							if fmt.Sprintf("%s:%s", msg.Name, msg.Version) == strings.Join(parts[2:], ":") {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("expected instruction %q not found in published messages", exp)
						}
					}
				}
			})
		})
	}
}

func TestReconcileWhenPreviousIsInProgress(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
		db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"aiven": "enabled"},
		})

		seeder.AddAssignment("feature-pending", "1.0.0", environment.Labels{"aiven": "enabled"})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		seeder.Reset()
		seeder.AddAssignment("feature-pending", "2.0.0", environment.Labels{})
		featureassignment.ChartDownloader = seeder.ChartDownloader()
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		count := db.countInstructions(ctx, t, "feature-pending", "2.0.0")
		if count != 0 {
			t.Errorf("count = %d; should not deploy v2 while v1 is in progress", count)
		}
	})
}

func TestReconcileWhenPreviousIsFailed(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
		db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"aiven": "enabled"},
		})

		seeder.AddAssignment("feature-failed", "1.0.0", environment.Labels{"aiven": "enabled"})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile #1: %v", err)
		}

		// Mark the instruction as failed (simulates naisd reporting failure).
		_, err := db.pool.Exec(ctx, `
			UPDATE deploy_instructions SET status = 'failed'
			WHERE feature_name = 'feature-failed' AND feature_version = '1.0.0'
		`)
		if err != nil {
			t.Fatalf("mark failed: %v", err)
		}

		// Reconcile again with same version — should not create a new instruction.
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile #2: %v", err)
		}

		count := db.countInstructions(ctx, t, "feature-failed", "1.0.0")
		if count != 1 {
			t.Errorf("count = %d; should not redeploy when previous failed and hash unchanged", count)
		}
	})
}

func TestReconcileDisabledFeature(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	t.Run("disabled feature is not deployed", func(t *testing.T) {
		forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
			db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
				"tenant1:dev":  {"kind": "tenant"},
				"tenant1:prod": {"kind": "tenant"},
			})

			var prodEnvID uuid.UUID
			err := db.pool.QueryRow(ctx, `SELECT e.id FROM environments e JOIN tenants t ON t.id = e.tenant_id WHERE t.name = 'tenant1' AND e.name = 'prod'`).Scan(&prodEnvID)
			if err != nil {
				t.Fatalf("get prod env id: %v", err)
			}
			_, err = db.pool.Exec(ctx, `INSERT INTO disabled_features (environment_id, feature) VALUES ($1, 'clamav')`, prodEnvID)
			if err != nil {
				t.Fatalf("disable feature: %v", err)
			}

			seeder.AddAssignment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
			if _, err := seeder.Seed(ctx); err != nil {
				t.Fatalf("seeding: %v", err)
			}

			if err := reconcile(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			if len(pub.msg) != 1 {
				t.Fatalf("pub.msg len = %d, want 1", len(pub.msg))
			}
			if pub.msg[0].Name != "clamav" {
				t.Errorf("msg name = %q, want clamav", pub.msg[0].Name)
			}
		})
	})

	t.Run("re-enabling allows future deploys", func(t *testing.T) {
		forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
			db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
				"tenant1:dev":  {"kind": "tenant"},
				"tenant1:prod": {"kind": "tenant"},
			})

			var prodEnvID uuid.UUID
			err := db.pool.QueryRow(ctx, `SELECT e.id FROM environments e JOIN tenants t ON t.id = e.tenant_id WHERE t.name = 'tenant1' AND e.name = 'prod'`).Scan(&prodEnvID)
			if err != nil {
				t.Fatalf("get prod env id: %v", err)
			}
			_, err = db.pool.Exec(ctx, `INSERT INTO disabled_features (environment_id, feature) VALUES ($1, 'clamav')`, prodEnvID)
			if err != nil {
				t.Fatalf("disable feature: %v", err)
			}

			seeder.AddAssignment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
			if _, err := seeder.Seed(ctx); err != nil {
				t.Fatalf("seeding: %v", err)
			}
			if err := reconcile(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			// Re-enable.
			_, err = db.pool.Exec(ctx, `DELETE FROM disabled_features WHERE environment_id = $1 AND feature = 'clamav'`, prodEnvID)
			if err != nil {
				t.Fatalf("delete disabled feature: %v", err)
			}

			// Deploy new version.
			seeder.Reset()
			pub.msg = nil
			seeder.AddAssignment("clamav", "0.2.0", environment.Labels{"kind": "tenant"})
			featureassignment.ChartDownloader = seeder.ChartDownloader()
			if _, err := seeder.Seed(ctx); err != nil {
				t.Fatalf("seeding: %v", err)
			}
			if err := reconcile(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			if len(pub.msg) != 2 {
				t.Fatalf("pub.msg len = %d, want 2 (both environments should receive deploy instructions)", len(pub.msg))
			}
		})
	})
}

func TestReconcileGlobalDeployment(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
		db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"tenant1:dev":        {"kind": "tenant"},
			"tenant1:prod":       {"kind": "tenant"},
			"tenant1:management": {"kind": "management"},
		})

		seeder.AddAssignment("global-tool", "1.0.0", environment.Labels{})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}

		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		if len(pub.msg) != 3 {
			t.Fatalf("pub.msg len = %d, want 3", len(pub.msg))
		}
		deployed := db.queryDeployedFeatures(ctx, t, "global-tool")
		if len(deployed) != 3 {
			t.Fatalf("deployed len = %d, want 3", len(deployed))
		}
		deployedSet := map[string]bool{}
		for _, d := range deployed {
			deployedSet[d] = true
		}
		if !deployedSet["tenant1:dev"] {
			t.Error("missing tenant1:dev in deployed")
		}
		if !deployedSet["tenant1:prod"] {
			t.Error("missing tenant1:prod in deployed")
		}
		if !deployedSet["tenant1:management"] {
			t.Error("missing tenant1:management in deployed")
		}
	})
}
