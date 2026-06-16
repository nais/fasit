//go:build integration_test

package reconciler_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/message"
)

// TestDeployInstructionValueOverrideChain verifies the precedence chain that
// produces the rendered helm values carried by message.DeployInstruction.Values:
//
//	computed < global config < env config override
//
// Chart defaults are intentionally not part of this chain: the reconciler never
// merges a chart's values.yaml into DeployInstruction.Values (helm applies those
// defaults itself), so a key that only has a chart default never appears here.
//
// It also pins the fasit metadata block and the .Env/.Management/.Configs
// template variables.
func TestDeployInstructionValueOverrideChain(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	h := newReconcileTest(ctx, t, container, dsn)
	h.createEnvs(
		tenantEnv{"acme", "mgmt", environment.Labels{"kind": "management"}},
		tenantEnv{"acme", "dev", environment.Labels{"kind": "tenant"}},
		tenantEnv{"acme", "clean", environment.Labels{"kind": "tenant"}},
	)

	tenant, err := environment.GetTenantByName(h.ctx, "acme")
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	mgmtEnv, err := environment.GetByName(h.ctx, tenant.ID, "mgmt")
	if err != nil {
		t.Fatalf("get mgmt env: %v", err)
	}
	devEnv, err := environment.GetByName(h.ctx, tenant.ID, "dev")
	if err != nil {
		t.Fatalf("get dev env: %v", err)
	}

	setEnvVal := func(envID uuid.UUID, key string, val any, secret bool) {
		t.Helper()
		b, err := json.Marshal(val)
		if err != nil {
			t.Fatalf("marshal env value %q: %v", key, err)
		}
		if err := environment.SetEnvironmentValue(h.ctx, envID, key, b, secret); err != nil {
			t.Fatalf("SetEnvironmentValue %q: %v", key, err)
		}
	}

	setEnvVal(mgmtEnv.ID, "mgmt_only", "from-mgmt", false)
	setEnvVal(devEnv.ID, "env_only", "from-env", false)
	setEnvVal(devEnv.ID, "secret_token", "s3cr3t", true)

	featureValues := feature.Values{
		"computedOnly":                {Computed: &feature.Computed{Template: "computed-result"}},
		"globalConfigOnly":            {Config: &feature.Config{Type: feature.ConfigTypeString}},
		"envConfigOnly":               {Config: &feature.Config{Type: feature.ConfigTypeString}},
		"globalBeatsChartDefault":     {Config: &feature.Config{Type: feature.ConfigTypeString}},
		"envBeatsGlobal":              {Config: &feature.Config{Type: feature.ConfigTypeString}},
		"configBeatsComputed":         {Config: &feature.Config{Type: feature.ConfigTypeString}, Computed: &feature.Computed{Template: "computed-value"}},
		"envConfigBeatsComputed":      {Config: &feature.Config{Type: feature.ConfigTypeString}, Computed: &feature.Computed{Template: "computed-value"}},
		"computedWinsWhenNoConfigSet": {Config: &feature.Config{Type: feature.ConfigTypeString}, Computed: &feature.Computed{Template: "computed-wins"}},
		"computedFromEnv":             {Computed: &feature.Computed{Template: `{{ .Env.env_only }}`}},
		"computedFromMgmt":            {Computed: &feature.Computed{Template: `{{ .Management.mgmt_only }}`}},
		"computedFromConfig":          {Computed: &feature.Computed{Template: `{{ .Configs.globalConfigOnly }}-suffix`}},
		"intConfig":                   {Config: &feature.Config{Type: feature.ConfigTypeInt}},
		"boolConfig":                  {Config: &feature.Config{Type: feature.ConfigTypeBool}},
		"computedFromSecretEnv":       {Computed: &feature.Computed{Template: `{{ .Env.secret_token }}`}},
		"ignoredForMgmt":              {Config: &feature.Config{Type: feature.ConfigTypeString}, IgnoreKind: []environment.EnvironmentKind{environment.EnvironmentKindManagement}},
	}

	chartDefaults := map[string]any{
		"globalBeatsChartDefault": "chart-default",
	}

	h.createAssignmentWithValues(
		"kitchen-sink", "1.0.0",
		environment.Labels{},
		[]environment.EnvironmentKind{environment.EnvironmentKindTenant, environment.EnvironmentKindManagement},
		featureValues,
		chartDefaults,
		"kitchen sink feature",
	)

	createConfig := func(key string, val any, envID *uuid.UUID) {
		t.Helper()
		b, err := json.Marshal(val)
		if err != nil {
			t.Fatalf("marshal config %q: %v", key, err)
		}
		c := feature.NewConfiguration{
			Feature:       "kitchen-sink",
			Key:           key,
			Value:         b,
			EnvironmentID: envID,
		}
		if envID != nil {
			_, err = feature.ConfigEnvCreate(h.ctx, c)
		} else {
			_, err = feature.ConfigGlobalCreate(h.ctx, c)
		}
		if err != nil {
			t.Fatalf("create config %q: %v", key, err)
		}
	}

	createConfig("globalConfigOnly", "global-val", nil)
	createConfig("globalBeatsChartDefault", "global-val", nil)
	createConfig("envBeatsGlobal", "global-val", nil)
	createConfig("configBeatsComputed", "global-val", nil)
	createConfig("intConfig", 42, nil)
	createConfig("boolConfig", true, nil)
	createConfig("ignoredForMgmt", "present", nil)
	createConfig("envConfigOnly", "env-val", &devEnv.ID)
	createConfig("envBeatsGlobal", "env-val", &devEnv.ID)
	createConfig("envConfigBeatsComputed", "env-val", &devEnv.ID)

	h.reconcile()

	instructions := map[string]message.DeployInstruction{}
	for _, msg := range h.pub.msg {
		instructions[h.instructionEnvName(msg.ID)] = msg
	}

	for _, env := range []string{"dev", "mgmt", "clean"} {
		if _, ok := instructions[env]; !ok {
			t.Errorf("expected deploy instruction for %s", env)
		}
	}

	t.Run("dev environment values", func(t *testing.T) {
		vals := instructions["dev"].Values

		if _, has := vals["chartDefault"]; has {
			t.Errorf("vals should not contain chartDefault")
		}
		assertValue(t, vals, "computedOnly", "computed-result")
		assertValue(t, vals, "globalConfigOnly", "global-val")
		assertValue(t, vals, "envConfigOnly", "env-val")
		assertValue(t, vals, "globalBeatsChartDefault", "global-val")
		assertValue(t, vals, "envBeatsGlobal", "env-val")
		assertValue(t, vals, "configBeatsComputed", "global-val")
		assertValue(t, vals, "envConfigBeatsComputed", "env-val")
		assertValue(t, vals, "computedWinsWhenNoConfigSet", "computed-wins")
		assertValue(t, vals, "computedFromEnv", "from-env")
		assertValue(t, vals, "computedFromMgmt", "from-mgmt")
		assertValue(t, vals, "computedFromConfig", "global-val-suffix")
		assertValue(t, vals, "computedFromSecretEnv", "s3cr3t")
		assertValue(t, vals, "ignoredForMgmt", "present")

		assertValue(t, vals, "intConfig", float64(42))
		assertValue(t, vals, "boolConfig", true)

		assertFasitMeta(t, vals, "dev", "tenant", "acme")
	})

	t.Run("mgmt environment values", func(t *testing.T) {
		vals := instructions["mgmt"].Values

		if _, has := vals["ignoredForMgmt"]; has {
			t.Errorf("vals should not contain ignoredForMgmt")
		}
		assertValue(t, vals, "globalConfigOnly", "global-val")
		assertValue(t, vals, "globalBeatsChartDefault", "global-val")
		assertValue(t, vals, "intConfig", float64(42))
		assertValue(t, vals, "boolConfig", true)

		assertValue(t, vals, "envBeatsGlobal", "global-val")
		assertValue(t, vals, "configBeatsComputed", "global-val")
		if _, has := vals["envConfigOnly"]; has {
			t.Errorf("vals should not contain envConfigOnly")
		}
		assertValue(t, vals, "envConfigBeatsComputed", "computed-value")
		assertValue(t, vals, "computedWinsWhenNoConfigSet", "computed-wins")
		assertValue(t, vals, "computedOnly", "computed-result")
		assertValue(t, vals, "computedFromEnv", "<no value>")
		assertValue(t, vals, "computedFromMgmt", "from-mgmt")
		assertValue(t, vals, "computedFromConfig", "global-val-suffix")
		assertValue(t, vals, "computedFromSecretEnv", "<no value>")

		assertFasitMeta(t, vals, "mgmt", "management", "acme")
	})

	t.Run("clean environment values", func(t *testing.T) {
		vals := instructions["clean"].Values

		if _, has := vals["chartDefault"]; has {
			t.Errorf("vals should not contain chartDefault")
		}
		assertValue(t, vals, "ignoredForMgmt", "present")

		assertValue(t, vals, "globalConfigOnly", "global-val")
		assertValue(t, vals, "globalBeatsChartDefault", "global-val")
		assertValue(t, vals, "envBeatsGlobal", "global-val")
		assertValue(t, vals, "configBeatsComputed", "global-val")
		assertValue(t, vals, "intConfig", float64(42))
		assertValue(t, vals, "boolConfig", true)

		if _, has := vals["envConfigOnly"]; has {
			t.Errorf("vals should not contain envConfigOnly")
		}
		assertValue(t, vals, "envConfigBeatsComputed", "computed-value")
		assertValue(t, vals, "computedWinsWhenNoConfigSet", "computed-wins")
		assertValue(t, vals, "computedOnly", "computed-result")
		assertValue(t, vals, "computedFromEnv", "<no value>")
		assertValue(t, vals, "computedFromMgmt", "from-mgmt")
		assertValue(t, vals, "computedFromConfig", "global-val-suffix")
		assertValue(t, vals, "computedFromSecretEnv", "<no value>")

		assertFasitMeta(t, vals, "clean", "tenant", "acme")
	})
}

// instructionEnvName resolves the environment name for a published deploy
// instruction via its persisted deploy_log row.
func (h *reconcileTest) instructionEnvName(diid uuid.UUID) string {
	h.t.Helper()
	var envName string
	err := h.pool.QueryRow(h.ctx, `
		SELECT e.name FROM deploy_log dl
		JOIN environments e ON e.id = dl.environment_id
		WHERE dl.diid = $1
		LIMIT 1
	`, diid).Scan(&envName)
	if err != nil {
		h.t.Fatalf("resolve env for instruction %s: %v", diid, err)
	}
	return envName
}

// assertValue compares a single key in the values map, unmarshaling
// json.RawMessage (config values) before comparison so they line up with the
// native Go types produced by computed templates.
func assertValue(t *testing.T, vals map[string]any, key string, expected any) {
	t.Helper()
	got, ok := vals[key]
	if !ok {
		t.Errorf("key %q not found in values map", key)
		return
	}

	if raw, ok := got.(json.RawMessage); ok {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Errorf("key %q: failed to unmarshal json.RawMessage: %v", key, err)
			return
		}
		got = decoded
	}

	if diff := cmp.Diff(expected, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("key %q mismatch (-want +got):\n%s", key, diff)
	}
}

// assertFasitMeta checks the injected fasit.{env,tenant} metadata block.
func assertFasitMeta(t *testing.T, vals map[string]any, envName, envKind, tenantName string) {
	t.Helper()
	fasit, ok := vals["fasit"].(map[string]any)
	if !ok {
		t.Fatalf("fasit block missing or wrong type: %T", vals["fasit"])
	}
	envMeta, ok := fasit["env"].(map[string]string)
	if !ok {
		t.Fatalf("fasit.env missing or wrong type: %T", fasit["env"])
	}
	if envMeta["name"] != envName {
		t.Errorf("fasit.env.name = %q, want %q", envMeta["name"], envName)
	}
	if envMeta["kind"] != envKind {
		t.Errorf("fasit.env.kind = %q, want %q", envMeta["kind"], envKind)
	}
	tenantMeta, ok := fasit["tenant"].(map[string]string)
	if !ok {
		t.Fatalf("fasit.tenant missing or wrong type: %T", fasit["tenant"])
	}
	if tenantMeta["name"] != tenantName {
		t.Errorf("fasit.tenant.name = %q, want %q", tenantMeta["name"], tenantName)
	}
}
