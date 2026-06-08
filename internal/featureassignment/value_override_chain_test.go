//go:build integration_test

package featureassignment_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/message"
)

// TestDeployInstructionValueOverrideChain verifies the precedence chain that
// produces DeployInstruction.Values:
//
//	chart default < computed < global config < env config override
func TestDeployInstructionValueOverrideChain(t *testing.T) {
	t.Skip("skip for now")
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	mgr := setupTestMgr(ctx, t, container, dsn, logger)

	// TODO: fix this: pub := mgr.publisher
	featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

	loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx = loadContext(ctx)

	mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
		"acme:mgmt":  {"kind": "management"},
		"acme:dev":   {"kind": "tenant"},
		"acme:clean": {"kind": "tenant"},
	})

	tenant, err := environment.GetTenantByName(ctx, "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgmtEnv, err := environment.GetByName(ctx, tenant.ID, "mgmt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	devEnv, err := environment.GetByName(ctx, tenant.ID, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	setEnvVal := func(envID uuid.UUID, key string, val any, secret bool) {
		t.Helper()
		b, err := json.Marshal(val)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := environment.SetEnvironmentValue(ctx, envID, key, b, secret); err != nil {
			t.Fatalf("SetEnvironmentValue: %v", err)
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

	mgr.seeder.AddAssignmentWithValues(
		"kitchen-sink", "1.0.0",
		environment.Labels{},
		[]environment.EnvironmentKind{environment.EnvironmentKindTenant, environment.EnvironmentKindManagement},
		featureValues,
		chartDefaults,
		"kitchen sink feature",
	)

	_, err = mgr.seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seed deployments: %v", err)
	}

	createConfig := func(key string, val any, envID *uuid.UUID) {
		t.Helper()
		b, err := json.Marshal(val)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		c := feature.NewConfiguration{
			Feature:       "kitchen-sink",
			Key:           key,
			Value:         b,
			EnvironmentID: envID,
		}
		if envID != nil {
			_, err = feature.ConfigEnvCreate(ctx, c)
		} else {
			_, err = feature.ConfigGlobalCreate(ctx, c)
		}
		if err != nil {
			t.Fatalf("create config %s: %v", key, err)
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

	/*
		TODO: fix for the new reconciler
		if err := featureassignment.GetManager(ctx).Reconcile(ctx); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}

	*/

	instructions := map[string]message.DeployInstruction{}
	/*
		TODO: fix this
			for _, msg := range pub.msg {
				row := mgr.db.pool.QueryRow(ctx,
					`SELECT e.name FROM deploy_instructions di
					 JOIN environments e ON e.id = di.environment_id
					 WHERE di.id = $1`, msg.ID)
				var envName string
				if err := row.Scan(&envName); err != nil {
					t.Fatalf("scan: %v", err)
				}
				instructions[envName] = msg
			}

	*/

	if _, ok := instructions["dev"]; !ok {
		t.Error("expected deploy instruction for dev")
	}
	if _, ok := instructions["mgmt"]; !ok {
		t.Error("expected deploy instruction for mgmt")
	}
	if _, ok := instructions["clean"]; !ok {
		t.Error("expected deploy instruction for clean")
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

		fasit, ok := vals["fasit"].(map[string]any)
		if !ok {
			t.Fatal("expected ok=true")
		}
		envMeta, ok := fasit["env"].(map[string]string)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if envMeta["name"] != "dev" {
			t.Errorf("got %v, want dev", envMeta["name"])
		}
		if envMeta["kind"] != "tenant" {
			t.Errorf("got %v, want tenant", envMeta["kind"])
		}
		tenantMeta, ok := fasit["tenant"].(map[string]string)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if tenantMeta["name"] != "acme" {
			t.Errorf("got %v, want acme", tenantMeta["name"])
		}
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

		fasit, ok := vals["fasit"].(map[string]any)
		if !ok {
			t.Fatal("expected ok=true")
		}
		envMeta, ok := fasit["env"].(map[string]string)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if envMeta["name"] != "mgmt" {
			t.Errorf("got %v, want mgmt", envMeta["name"])
		}
		if envMeta["kind"] != "management" {
			t.Errorf("got %v, want management", envMeta["kind"])
		}
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

		fasit, ok := vals["fasit"].(map[string]any)
		if !ok {
			t.Fatal("expected ok=true")
		}
		envMeta, ok := fasit["env"].(map[string]string)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if envMeta["name"] != "clean" {
			t.Errorf("got %v, want clean", envMeta["name"])
		}
		if envMeta["kind"] != "tenant" {
			t.Errorf("got %v, want tenant", envMeta["kind"])
		}
	})
}

// assertValue compares a single key in the values map, unmarshaling
// json.RawMessage when needed.
func assertValue(t *testing.T, vals map[string]any, key string, expected any) {
	t.Helper()
	got, ok := vals[key]
	if !ok {
		t.Errorf("key %q not found in values map", key)
		return
	}

	// json.RawMessage values need to be unmarshaled for comparison.
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
