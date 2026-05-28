//go:build integration_test && reconciler_bench

package deployment_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

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

// TestReconcileWorkerPoolScaling measures compute phase duration with different
// worker pool sizes to identify whether rendering or map deep-copy dominates.
func TestReconcileWorkerPoolScaling(t *testing.T) {
	ctx := context.Background()
	_, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	const (
		numFeatures = 100
		numTenants  = 10
	)
	envKinds := []string{"dev", "staging", "prod"}

	// Only use new reconciler for this test.
	logger, _ := test.NewNullLogger()
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Wire context for seeding.
	oldPub := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return &sqlPublisher{pool: pool}
	}
	loadContext, err := contextloader.NewLoaderFunc(pool, oldPub, meter, logger)
	if err != nil {
		t.Fatalf("loader: %v", err)
	}
	ctx = loadContext(ctx)

	seeder := deploymenttest.NewSeeder()

	// Create environments.
	db := &reconcileDB{Db{t: t, pool: pool}}
	envsToCreate := map[string]environment.Labels{}
	for ti := range numTenants {
		tenant := fmt.Sprintf("tenant-%02d", ti)
		for _, kind := range envKinds {
			key := fmt.Sprintf("%s:%s", tenant, kind)
			envsToCreate[key] = environment.Labels{"kind": "tenant"}
		}
	}
	db.createTenantsAndEnvironments(ctx, envsToCreate)

	// Seed features.
	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		values := model.Values{
			"setting_a":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting A"},
			"setting_b":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting B"},
			"computed_name": {Computed: &model.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
			"computed_cfg":  {Computed: &model.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
		}
		defaults := map[string]any{
			"setting_a": fmt.Sprintf("default-a-%d", fi),
			"setting_b": fmt.Sprintf("default-b-%d", fi),
		}
		seeder.AddDeploymentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
	}
	deployment.ChartDownloader = seeder.ChartDownloader()
	if _, err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// Create global configs.
	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		for _, key := range []string{"setting_a", "setting_b"} {
			b, _ := json.Marshal(fmt.Sprintf("global-%s-%d", key, fi))
			if _, err := feature.ConfigGlobalCreate(ctx, model.NewConfiguration{
				Feature: name, Key: key, Value: b,
			}); err != nil {
				t.Fatalf("create config: %v", err)
			}
		}
	}

	// Test different worker pool sizes.
	workerCounts := []int{0, 1, 2, 4, 8, 16, 32, 64, 128}

	for _, workers := range workerCounts {
		label := "unlimited"
		if workers > 0 {
			label = fmt.Sprintf("%d", workers)
		}
		t.Run("workers="+label, func(t *testing.T) {
			rec, err := reconciler.New(pool, meter, logger)
			if err != nil {
				t.Fatalf("create reconciler: %v", err)
			}
			rec.Workers = workers

			result, err := rec.ComputeDesiredState(ctx)
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			deployCount := 0
			for _, d := range result.Decisions {
				if d.Action == reconciler.ActionDeploy {
					deployCount++
				}
			}

			t.Logf("workers=%-10s  fetch=%-10s  compute=%-10s  total=%-10s  decisions=%d  deploys=%d",
				label,
				result.FetchDur.Round(time.Millisecond),
				result.ComputeDur.Round(time.Millisecond),
				(result.FetchDur + result.ComputeDur).Round(time.Millisecond),
				len(result.Decisions),
				deployCount,
			)
		})
	}
}
