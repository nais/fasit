//go:build integration_test

package reconciler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/graph/model"
)

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

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
			timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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
	container, dsn := startPostgresWithSnapshot(ctx, t)

	timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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
	container, dsn := startPostgresWithSnapshot(ctx, t)

	timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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

		_, err := db.pool.Exec(ctx, `
			UPDATE deploy_instructions SET status = 'failed'
			WHERE feature_name = 'feature-failed' AND feature_version = '1.0.0'
		`)
		if err != nil {
			t.Fatalf("mark failed: %v", err)
		}

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
	container, dsn := startPostgresWithSnapshot(ctx, t)

	t.Run("disabled feature is not deployed", func(t *testing.T) {
		timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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
		timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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

			_, err = db.pool.Exec(ctx, `DELETE FROM disabled_features WHERE environment_id = $1 AND feature = 'clamav'`, prodEnvID)
			if err != nil {
				t.Fatalf("delete disabled feature: %v", err)
			}

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
	container, dsn := startPostgresWithSnapshot(ctx, t)

	timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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

// TestReconcileRealisticScale seeds 100 features × 2 deployments each across
// 10 tenants × 3 environments (30 envs), with global and per-environment
// config overrides, exercising the full config merge + helm render path.
func TestReconcileRealisticScale(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	const (
		numFeatures = 100
		numTenants  = 10
	)
	envKinds := []string{"dev", "staging", "prod"}

	timedReconcileTest(t, ctx, container, dsn, func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder) {
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
			seeder.AddAssignmentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
			targetTenant := fmt.Sprintf("tenant-%02d", fi%numTenants)
			seeder.AddAssignmentWithValues(name, fmt.Sprintf("2.%d.0", fi), environment.Labels{"kind": "tenant", "tenant": targetTenant}, nil, values, defaults, fmt.Sprintf("Feature %d targeted", fi))
		}

		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seeding: %v", err)
		}

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

		var depCount, globalCfgCount, envCfgCount int
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM feature_assignments`).Scan(&depCount)
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
		if totalInstructions != numFeatures*len(allEnvs) {
			t.Errorf("deployed instructions = %d, want %d", totalInstructions, numFeatures*len(allEnvs))
		}

		// --- Second pass: deploy a new version of ONE feature only. ---
		changedFeature := "feature-042"
		seeder.Reset()
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
			seeder.AddAssignmentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
			targetTenant := fmt.Sprintf("tenant-%02d", fi%numTenants)
			seeder.AddAssignmentWithValues(name, fmt.Sprintf("2.%d.0", fi), environment.Labels{"kind": "tenant", "tenant": targetTenant}, nil, values, defaults, fmt.Sprintf("Feature %d targeted", fi))
		}
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
		seeder.AddAssignmentWithValues(changedFeature, "3.0.0", environment.Labels{"kind": "tenant"}, nil, changedValues, changedDefaults, "Feature 42 updated")
		featureassignment.ChartDownloader = seeder.ChartDownloader()

		if _, err := featureassignment.Create(ctx, featureassignment.CreateFeatureAssignment{
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

		expectedChanged := len(allEnvs) - len(envKinds) // 30 - 3 = 27
		var newInstructions int
		db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM deploy_instructions WHERE status = 'deployed' AND feature_name = $1 AND feature_version = '3.0.0'`, changedFeature).Scan(&newInstructions)
		t.Logf("new deployed instructions for %s v3.0.0: %d", changedFeature, newInstructions)
		if newInstructions != expectedChanged {
			t.Errorf("new instructions = %d, want %d", newInstructions, expectedChanged)
		}
		if len(pub.msg) != expectedChanged {
			t.Errorf("published messages = %d, want %d (only the changed feature should produce new messages)", len(pub.msg), expectedChanged)
		}
	})
}
