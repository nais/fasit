//go:build integration_test

package deployment_test

import (
	"context"
	"encoding/json"
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
	"github.com/nais/fasit/internal/feature"
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
	rec, err = reconciler.New(reconcilersql.New(pool), newPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create reconciler: %v", err)
	}
	newReconcile = func(ctx context.Context) error {
		return rec.Reconcile(ctx)
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
			ctx, db, pub, oldR, newR, seeder, rec := setupReconcileTest(ctx, t, container, dsn)
			deployment.ChartDownloader = seeder.ChartDownloader()

			isNew := impl.name == "new"
			call := 0
			raw := impl.pick(oldR, newR)
			timed := func(ctx context.Context) error {
				call++
				start := time.Now()
				err := raw(ctx)
				t.Logf("reconcile #%d took %s", call, time.Since(start))
				if isNew {
					t.Logf("  phases: fetch=%s render=%s io=%s", rec.LastFetchDur, rec.LastRenderDur, rec.LastIODur)
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

// TestReconcileRealisticScale seeds 100 features × 2 deployments each across
// 10 tenants × 3 environments (30 envs), with global and per-environment
// config overrides, exercising the full config merge + helm render path.
func TestReconcileRealisticScale(t *testing.T) {
	ctx := context.Background()
	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	const (
		numFeatures = 100
		numTenants  = 10
	)
	envKinds := []string{"dev", "staging", "prod"}

	forEachReconciler(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *deploymenttest.Seeder) {
		// Create 10 tenants × 3 environments = 30 environments.
		envsToCreate := map[string]environment.Labels{}
		for ti := range numTenants {
			tenant := fmt.Sprintf("tenant-%02d", ti)
			for _, kind := range envKinds {
				key := fmt.Sprintf("%s:%s", tenant, kind)
				envsToCreate[key] = environment.Labels{"kind": "tenant"}
			}
		}
		db.createTenantsAndEnvironments(ctx, envsToCreate)

		// Collect environment IDs for config overrides.
		type envInfo struct {
			id   uuid.UUID
			name string
		}
		var allEnvs []envInfo
		rows, err := db.pool.Query(ctx, `SELECT id, name FROM environments ORDER BY name`)
		if err != nil {
			t.Fatalf("list envs: %v", err)
		}
		for rows.Next() {
			var e envInfo
			if err := rows.Scan(&e.id, &e.name); err != nil {
				t.Fatalf("scan env: %v", err)
			}
			allEnvs = append(allEnvs, e)
		}
		rows.Close()

		// Seed 100 features with 2 deployments each.
		// Each feature has 5 config-backed values and 3 computed values.
		for fi := range numFeatures {
			name := fmt.Sprintf("feature-%03d", fi)

			values := model.Values{
				"setting_a": {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting A"},
				"setting_b": {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting B"},
				"setting_c": {Config: &model.Config{Type: model.ConfigTypeInt}, DisplayName: "Setting C"},
				"toggle":    {Config: &model.Config{Type: model.ConfigTypeBool}, DisplayName: "Toggle"},
				"secret_key": {Config: &model.Config{Type: model.ConfigTypeString, Secret: true}, DisplayName: "Secret Key"},
				"computed_name": {Computed: &model.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
				"computed_full": {Computed: &model.Computed{Template: `"{{ .Env.name }}.{{ .Tenant.Name }}.example.com"`}},
				"computed_cfg":  {Computed: &model.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
			}
			defaults := map[string]any{
				"setting_a":  fmt.Sprintf("default-a-%d", fi),
				"setting_b":  fmt.Sprintf("default-b-%d", fi),
				"setting_c":  fi * 10,
				"toggle":     fi%2 == 0,
				"secret_key": fmt.Sprintf("secret-%d", fi),
			}

			// Broad deployment targeting all tenants.
			seeder.AddDeploymentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
			// Narrower deployment for one specific tenant.
			targetTenant := fmt.Sprintf("tenant-%02d", fi%numTenants)
			seeder.AddDeploymentWithValues(name, fmt.Sprintf("2.%d.0", fi), environment.Labels{"kind": "tenant", "tenant": targetTenant}, nil, values, defaults, fmt.Sprintf("Feature %d targeted", fi))
		}

		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}

		// Create global configs for every feature (5 keys each = 500 global rows).
		for fi := range numFeatures {
			name := fmt.Sprintf("feature-%03d", fi)
			for _, key := range []string{"setting_a", "setting_b", "setting_c", "toggle", "secret_key"} {
				var val any
				switch key {
				case "setting_a":
					val = fmt.Sprintf("global-a-%d", fi)
				case "setting_b":
					val = fmt.Sprintf("global-b-%d", fi)
				case "setting_c":
					val = fi * 100
				case "toggle":
					val = fi%3 == 0
				case "secret_key":
					val = fmt.Sprintf("global-secret-%d", fi)
				}
				b, _ := json.Marshal(val)
				if _, err := feature.ConfigGlobalCreate(ctx, model.NewConfiguration{
					Feature: name,
					Key:     key,
					Value:   b,
				}); err != nil {
					t.Fatalf("create global config %s/%s: %v", name, key, err)
				}
			}
		}

		// Create per-environment config overrides.
		// Every "prod" env overrides setting_a and toggle for every feature.
		// Every "staging" env overrides setting_b for every feature.
		// That's ~100*10 prod overrides + ~100*10 staging overrides = ~2000 env config rows.
		for _, env := range allEnvs {
			for fi := range numFeatures {
				name := fmt.Sprintf("feature-%03d", fi)
				envID := env.id

				if env.name == "prod" {
					for _, key := range []string{"setting_a", "toggle"} {
						var val any
						if key == "setting_a" {
							val = fmt.Sprintf("prod-a-%d-%s", fi, envID)
						} else {
							val = false
						}
						b, _ := json.Marshal(val)
						if _, err := feature.ConfigEnvCreate(ctx, model.NewConfiguration{
							EnvironmentID: &envID,
							Feature:       name,
							Key:           key,
							Value:         b,
						}); err != nil {
							t.Fatalf("create env config %s/%s/%s: %v", env.name, name, key, err)
						}
					}
				}

				if env.name == "staging" {
					b, _ := json.Marshal(fmt.Sprintf("staging-b-%d-%s", fi, envID))
					if _, err := feature.ConfigEnvCreate(ctx, model.NewConfiguration{
						EnvironmentID: &envID,
						Feature:       name,
						Key:           "setting_b",
						Value:         b,
					}); err != nil {
						t.Fatalf("create env config %s/%s/setting_b: %v", env.name, name, err)
					}
				}
			}
		}

		// Verify seed counts.
		var depCount, globalCfgCount, envCfgCount int
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM deployments`).Scan(&depCount)
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM configurations_global`).Scan(&globalCfgCount)
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM configurations_environment`).Scan(&envCfgCount)
		t.Logf("seeded: %d deployments, %d environments, %d global configs, %d env configs",
			depCount, len(allEnvs), globalCfgCount, envCfgCount)

		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		var totalInstructions int
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM deploy_instructions WHERE status = 'deployed'`).Scan(&totalInstructions)
		t.Logf("deployed instructions: %d", totalInstructions)
		assert.Equal(t, numFeatures*len(allEnvs), totalInstructions)

		// --- Second pass: deploy a new version of ONE feature only. ---
		// The reconciler must re-evaluate everything but only deploy 30 new
		// instructions (one per environment) for the changed feature.
		changedFeature := "feature-042"
		seeder.Reset()
		// Re-register all existing features so ChartDownloader still resolves them.
		for fi := range numFeatures {
			name := fmt.Sprintf("feature-%03d", fi)
			values := model.Values{
				"setting_a":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting A"},
				"setting_b":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting B"},
				"setting_c":     {Config: &model.Config{Type: model.ConfigTypeInt}, DisplayName: "Setting C"},
				"toggle":        {Config: &model.Config{Type: model.ConfigTypeBool}, DisplayName: "Toggle"},
				"secret_key":    {Config: &model.Config{Type: model.ConfigTypeString, Secret: true}, DisplayName: "Secret Key"},
				"computed_name": {Computed: &model.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
				"computed_full": {Computed: &model.Computed{Template: `"{{ .Env.name }}.{{ .Tenant.Name }}.example.com"`}},
				"computed_cfg":  {Computed: &model.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
			}
			defaults := map[string]any{
				"setting_a":  fmt.Sprintf("default-a-%d", fi),
				"setting_b":  fmt.Sprintf("default-b-%d", fi),
				"setting_c":  fi * 10,
				"toggle":     fi%2 == 0,
				"secret_key": fmt.Sprintf("secret-%d", fi),
			}
			// Keep the existing versions so the downloader can resolve them.
			seeder.AddDeploymentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
			targetTenant := fmt.Sprintf("tenant-%02d", fi%numTenants)
			seeder.AddDeploymentWithValues(name, fmt.Sprintf("2.%d.0", fi), environment.Labels{"kind": "tenant", "tenant": targetTenant}, nil, values, defaults, fmt.Sprintf("Feature %d targeted", fi))
		}
		// Add the NEW version for the changed feature.
		changedValues := model.Values{
			"setting_a":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting A"},
			"setting_b":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting B"},
			"setting_c":     {Config: &model.Config{Type: model.ConfigTypeInt}, DisplayName: "Setting C"},
			"toggle":        {Config: &model.Config{Type: model.ConfigTypeBool}, DisplayName: "Toggle"},
			"secret_key":    {Config: &model.Config{Type: model.ConfigTypeString, Secret: true}, DisplayName: "Secret Key"},
			"computed_name": {Computed: &model.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
			"computed_full": {Computed: &model.Computed{Template: `"{{ .Env.name }}.{{ .Tenant.Name }}.example.com"`}},
			"computed_cfg":  {Computed: &model.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
		}
		changedDefaults := map[string]any{
			"setting_a":  "changed-default-a",
			"setting_b":  "changed-default-b",
			"setting_c":  9999,
			"toggle":     true,
			"secret_key": "changed-secret",
		}
		seeder.AddDeploymentWithValues(changedFeature, "3.0.0", environment.Labels{"kind": "tenant"}, nil, changedValues, changedDefaults, "Feature 42 updated")
		deployment.ChartDownloader = seeder.ChartDownloader()

		// Seed the new deployment into the database.
		if _, err := deployment.Create(ctx, deployment.CreateDeployment{
			Chart:   "oci://" + changedFeature,
			Version: "3.0.0",
			Target:  environment.Labels{"kind": "tenant"},
		}); err != nil {
			t.Fatalf("create changed deployment: %v", err)
		}

		pub.msg = nil
		t.Logf("--- second pass: 1 feature changed (%s v3.0.0) ---", changedFeature)
		if err := reconcile(ctx); err != nil {
			t.Fatalf("reconcile (pass 2): %v", err)
		}
		t.Logf("published %d messages in pass 2", len(pub.msg))

		// v3.0.0 targets {kind:tenant} broadly. feature-042 also has a
		// 2.42.0 deployment targeting {kind:tenant, tenant:tenant-02} which
		// is more specific and wins in tenant-02's 3 environments.
		// So v3.0.0 deploys to 30 - 3 = 27 environments.
		expectedChanged := len(allEnvs) - len(envKinds) // 30 - 3 = 27
		var newInstructions int
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM deploy_instructions WHERE status = 'deployed' AND feature_name = $1 AND feature_version = '3.0.0'`, changedFeature).Scan(&newInstructions)
		t.Logf("new deployed instructions for %s v3.0.0: %d", changedFeature, newInstructions)
		assert.Equal(t, expectedChanged, newInstructions)
		assert.Len(t, pub.msg, expectedChanged, "only the changed feature should produce new messages")
	})
}
