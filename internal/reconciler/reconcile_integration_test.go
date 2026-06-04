//go:build integration_test

package reconciler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
)

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	envs := []tenantEnv{
		{"test-partner", "dev", environment.Labels{}},
		{"test-partner", "prod", environment.Labels{"featuretoggle": "enabled"}},
		{"nav", "dev", environment.Labels{"aiven": "enabled"}},
		{"nav", "management", environment.Labels{"kind": "management"}},
	}

	type featureInput struct {
		name, version string
		dependencies  []string
		target        environment.Labels
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
			h := newReconcileTest(ctx, t, container, dsn)
			h.createEnvs(envs...)
			for _, input := range tc.deploymentsToCreate {
				h.createAssignment(input.name, input.version, input.target, input.dependencies...)
			}

			for _, expected := range tc.reconcileResults {
				h.reconcile()
				h.requireDeployed(expected...)
				h.requirePublished(len(expected))
			}
		})
	}
}

func TestReconcileWhenPreviousIsInProgress(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	h := newReconcileTest(ctx, t, container, dsn)
	h.createEnvs(tenantEnv{"nav", "dev", environment.Labels{"aiven": "enabled"}})

	h.createAssignment("feature-pending", "1.0.0", environment.Labels{"aiven": "enabled"})
	h.reconcile()

	h.createAssignment("feature-pending", "2.0.0", environment.Labels{})
	h.reconcile()

	if count := h.countInstructions("feature-pending", "2.0.0"); count != 0 {
		t.Errorf("count = %d; should not deploy v2 while v1 is in progress", count)
	}
}

func TestReconcileWhenPreviousIsFailed(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	h := newReconcileTest(ctx, t, container, dsn)
	h.createEnvs(tenantEnv{"nav", "dev", environment.Labels{"aiven": "enabled"}})

	h.createAssignment("feature-failed", "1.0.0", environment.Labels{"aiven": "enabled"})
	h.reconcile()

	_, err := h.pool.Exec(ctx, `
		UPDATE deploy_instructions SET status = 'failed'
		WHERE feature_name = 'feature-failed' AND feature_version = '1.0.0'
	`)
	if err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	h.reconcile()

	if count := h.countInstructions("feature-failed", "1.0.0"); count != 1 {
		t.Errorf("count = %d; should not redeploy when previous failed and hash unchanged", count)
	}
}

func TestReconcileDisabledFeature(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	disableFeature := func(h *reconcileTest, tenant, env, feature string) {
		t.Helper()
		var envID uuid.UUID
		err := h.pool.QueryRow(ctx, `SELECT e.id FROM environments e JOIN tenants t ON t.id = e.tenant_id WHERE t.name = $1 AND e.name = $2`, tenant, env).Scan(&envID)
		if err != nil {
			t.Fatalf("get env id: %v", err)
		}
		if _, err := h.pool.Exec(ctx, `INSERT INTO disabled_features (environment_id, feature) VALUES ($1, $2)`, envID, feature); err != nil {
			t.Fatalf("disable feature: %v", err)
		}
	}

	t.Run("disabled feature is not deployed", func(t *testing.T) {
		h := newReconcileTest(ctx, t, container, dsn)
		h.createEnvs(
			tenantEnv{"tenant1", "dev", environment.Labels{"kind": "tenant"}},
			tenantEnv{"tenant1", "prod", environment.Labels{"kind": "tenant"}},
		)
		disableFeature(h, "tenant1", "prod", "clamav")

		h.createAssignment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
		h.reconcile()

		h.requirePublished(1)
		if h.pub.msg[0].Name != "clamav" {
			t.Errorf("msg name = %q, want clamav", h.pub.msg[0].Name)
		}
	})

	t.Run("re-enabling allows future deploys", func(t *testing.T) {
		h := newReconcileTest(ctx, t, container, dsn)
		h.createEnvs(
			tenantEnv{"tenant1", "dev", environment.Labels{"kind": "tenant"}},
			tenantEnv{"tenant1", "prod", environment.Labels{"kind": "tenant"}},
		)
		disableFeature(h, "tenant1", "prod", "clamav")

		h.createAssignment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
		h.reconcile()

		if _, err := h.pool.Exec(ctx, `DELETE FROM disabled_features WHERE feature = 'clamav'`); err != nil {
			t.Fatalf("delete disabled feature: %v", err)
		}

		h.pub.msg = nil
		h.createAssignment("clamav", "0.2.0", environment.Labels{"kind": "tenant"})
		h.reconcile()

		h.requirePublished(2)
	})
}

func TestReconcileGlobalDeployment(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	h := newReconcileTest(ctx, t, container, dsn)
	h.createEnvs(
		tenantEnv{"tenant1", "dev", environment.Labels{"kind": "tenant"}},
		tenantEnv{"tenant1", "prod", environment.Labels{"kind": "tenant"}},
		tenantEnv{"tenant1", "management", environment.Labels{"kind": "management"}},
	)

	h.createAssignment("global-tool", "1.0.0", environment.Labels{})
	h.reconcile()

	h.requirePublished(3)
	deployed := h.deployedFeatures("global-tool")
	want := []string{"tenant1:dev", "tenant1:management", "tenant1:prod"}
	if !equalStringSet(deployed, want) {
		t.Errorf("deployed = %v, want %v", deployed, want)
	}
}

func equalStringSet(got, want []string) bool {
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	if len(set) != len(want) {
		return false
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
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

	h := newReconcileTest(ctx, t, container, dsn)

	// Create 10 tenants × 3 environments = 30 environments.
	var envs []tenantEnv
	for ti := range numTenants {
		tenant := fmt.Sprintf("tenant-%02d", ti)
		for _, kind := range envKinds {
			envs = append(envs, tenantEnv{tenant, kind, environment.Labels{"kind": "tenant"}})
		}
	}
	h.createEnvs(envs...)

	type envInfo struct {
		id   uuid.UUID
		name string
	}
	var allEnvs []envInfo
	rows, err := h.pool.Query(ctx, `SELECT id, name FROM environments ORDER BY name`)
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

	standardValues := func() model.Values {
		return model.Values{
			"setting_a":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting A"},
			"setting_b":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting B"},
			"setting_c":     {Config: &model.Config{Type: model.ConfigTypeInt}, DisplayName: "Setting C"},
			"toggle":        {Config: &model.Config{Type: model.ConfigTypeBool}, DisplayName: "Toggle"},
			"secret_key":    {Config: &model.Config{Type: model.ConfigTypeString, Secret: true}, DisplayName: "Secret Key"},
			"computed_name": {Computed: &model.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
			"computed_full": {Computed: &model.Computed{Template: `"{{ .Env.name }}.{{ .Tenant.Name }}.example.com"`}},
			"computed_cfg":  {Computed: &model.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
		}
	}

	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		defaults := map[string]any{
			"setting_a":  fmt.Sprintf("default-a-%d", fi),
			"setting_b":  fmt.Sprintf("default-b-%d", fi),
			"setting_c":  fi * 10,
			"toggle":     fi%2 == 0,
			"secret_key": fmt.Sprintf("secret-%d", fi),
		}
		h.createAssignmentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, standardValues(), defaults, fmt.Sprintf("Feature %d", fi))
		targetTenant := fmt.Sprintf("tenant-%02d", fi%numTenants)
		h.createAssignmentWithValues(name, fmt.Sprintf("2.%d.0", fi), environment.Labels{"kind": "tenant", "tenant": targetTenant}, nil, standardValues(), defaults, fmt.Sprintf("Feature %d targeted", fi))
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
			if _, err := feature.ConfigGlobalCreate(h.ctx, model.NewConfiguration{
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
					if _, err := feature.ConfigEnvCreate(h.ctx, model.NewConfiguration{
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
				if _, err := feature.ConfigEnvCreate(h.ctx, model.NewConfiguration{
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
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM feature_assignments`).Scan(&depCount)
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM configurations_global`).Scan(&globalCfgCount)
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM configurations_environment`).Scan(&envCfgCount)
	t.Logf("seeded: %d deployments, %d environments, %d global configs, %d env configs",
		depCount, len(allEnvs), globalCfgCount, envCfgCount)

	h.reconcile()

	var totalInstructions int
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM deploy_instructions WHERE status = 'deployed'`).Scan(&totalInstructions)
	t.Logf("deployed instructions: %d", totalInstructions)
	if totalInstructions != numFeatures*len(allEnvs) {
		t.Errorf("deployed instructions = %d, want %d", totalInstructions, numFeatures*len(allEnvs))
	}

	// --- Second pass: deploy a new version of ONE feature only. ---
	changedFeature := "feature-042"
	changedDefaults := map[string]any{
		"setting_a":  "changed-default-a",
		"setting_b":  "changed-default-b",
		"setting_c":  9999,
		"toggle":     true,
		"secret_key": "changed-secret",
	}
	h.pub.msg = nil
	t.Logf("--- second pass: 1 feature changed (%s v3.0.0) ---", changedFeature)
	h.createAssignmentWithValues(changedFeature, "3.0.0", environment.Labels{"kind": "tenant"}, nil, standardValues(), changedDefaults, "Feature 42 updated")
	h.reconcile()
	t.Logf("published %d messages in pass 2", len(h.pub.msg))

	expectedChanged := len(allEnvs) - len(envKinds) // 30 - 3 = 27
	var newInstructions int
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM deploy_instructions WHERE status = 'deployed' AND feature_name = $1 AND feature_version = '3.0.0'`, changedFeature).Scan(&newInstructions)
	t.Logf("new deployed instructions for %s v3.0.0: %d", changedFeature, newInstructions)
	if newInstructions != expectedChanged {
		t.Errorf("new instructions = %d, want %d", newInstructions, expectedChanged)
	}
	if len(h.pub.msg) != expectedChanged {
		t.Errorf("published messages = %d, want %d (only the changed feature should produce new messages)", len(h.pub.msg), expectedChanged)
	}
}
