//go:build integration_test

package deployment_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
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
	oldReconcile reconcileFunc,
	newReconcile reconcileFunc,
	seeder *deploymenttest.Seeder,
	rec *reconciler.Reconciler,
	lastResult **reconciler.DesiredState,
) {
	t.Helper()
	logger, _ := test.NewNullLogger()

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
	seeder = deploymenttest.NewSeeder()

	// Wire old reconciler via context loader.
	oldPub := func(topicID string, log logrus.FieldLogger) deployment.Publisher { return pub }
	loadContext, err := contextloader.NewLoaderFunc(pool, oldPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}
	newCtx = loadContext(ctx)

	oldReconcile = func(ctx context.Context) error {
		return deployment.GetManager(ctx).Reconcile(ctx)
	}

	// Wire new reconciler.
	newPub := func(topicID string, log logrus.FieldLogger) reconciler.Publisher { return pub }
	recQuerier := reconcilersql.New(pool)
	dispatcher, err := reconciler.NewDBDispatcher(recQuerier, newPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create result writer: %v", err)
	}
	rec, err = reconciler.New(recQuerier, meter, logger)
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
func forEachReconciler(
	t *testing.T,
	ctx context.Context,
	container *postgres.PostgresContainer,
	dsn string,
	fn func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder),
) {
	t.Helper()
	for _, impl := range []struct {
		name string
		pick func(old, new reconcileFunc) reconcileFunc
	}{
		{"old", func(old, _ reconcileFunc) reconcileFunc { return old }},
		{"new", func(_, new reconcileFunc) reconcileFunc { return new }},
	} {
		t.Run(impl.name, func(t *testing.T) {
			ctx, db, pub, oldR, newR, seeder, _, lastResultPtr := setupReconcileTest(ctx, t, container, dsn)
			deployment.ChartDownloader = seeder.ChartDownloader()

			isNew := impl.name == "new"
			call := 0
			raw := impl.pick(oldR, newR)
			timed := func(ctx context.Context) error {
				call++
				start := time.Now()
				err := raw(ctx)
				elapsed := time.Since(start)
				t.Logf("reconcile #%d took %s", call, elapsed)
				if isNew && *lastResultPtr != nil {
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
			forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder) {
				db.createTenantsAndEnvironments(ctx, envsToCreate)
				for _, input := range tc.deploymentsToCreate {
					seeder.AddDeployment(input.name, input.version, input.target, input.dependencies...)
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

					assert.Equal(t, sorted, instructions)
					assert.Len(t, pub.msg, len(expected))

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

	forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder) {
		db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"aiven": "enabled"},
		})

		seeder.AddDeployment("feature-pending", "1.0.0", environment.Labels{"aiven": "enabled"})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		seeder.Reset()
		seeder.AddDeployment("feature-pending", "2.0.0", environment.Labels{})
		deployment.ChartDownloader = seeder.ChartDownloader()
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		count := db.countInstructions(ctx, t, "feature-pending", "2.0.0")
		assert.Equal(t, 0, count, "should not deploy v2 while v1 is in progress")
	})
}

func TestReconcileDisabledFeature(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	t.Run("disabled feature is not deployed", func(t *testing.T) {
		forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder) {
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

			seeder.AddDeployment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
			if _, err := seeder.Seed(ctx); err != nil {
				t.Fatalf("seeding: %v", err)
			}

			if err := reconcile(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			assert.Len(t, pub.msg, 1)
			assert.Equal(t, "clamav", pub.msg[0].Name)
		})
	})

	t.Run("re-enabling allows future deploys", func(t *testing.T) {
		forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder) {
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

			seeder.AddDeployment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
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
			seeder.AddDeployment("clamav", "0.2.0", environment.Labels{"kind": "tenant"})
			deployment.ChartDownloader = seeder.ChartDownloader()
			if _, err := seeder.Seed(ctx); err != nil {
				t.Fatalf("seeding: %v", err)
			}
			if err := reconcile(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			assert.Len(t, pub.msg, 2, "both environments should receive deploy instructions")
		})
	})
}

func TestReconcileGlobalDeployment(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder) {
		db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"tenant1:dev":        {"kind": "tenant"},
			"tenant1:prod":       {"kind": "tenant"},
			"tenant1:management": {"kind": "management"},
		})

		seeder.AddDeployment("global-tool", "1.0.0", environment.Labels{})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}

		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		assert.Len(t, pub.msg, 3)
		deployed := db.queryDeployedFeatures(ctx, t, "global-tool")
		assert.Len(t, deployed, 3)
		assert.Contains(t, deployed, "tenant1:dev")
		assert.Contains(t, deployed, "tenant1:prod")
		assert.Contains(t, deployed, "tenant1:management")
	})
}
