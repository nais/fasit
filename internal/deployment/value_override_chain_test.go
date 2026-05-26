//go:build integration_test

package deployment_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeployInstructionValueOverrideChain verifies the precedence chain that
// produces DeployInstruction.Values:
//
//	chart default < computed < global config < env config override
func TestDeployInstructionValueOverrideChain(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t)
	require.NoError(t, err, "start postgres")

	mgr := setupTestMgr(ctx, t, container, dsn, logger)

	pub := mgr.publisher
	deployment.ChartDownloader = mgr.seeder.ChartDownloader()

	newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return pub
	}
	loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
	require.NoError(t, err)
	ctx = loadContext(ctx)

	mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
		"acme:mgmt":  {"kind": "management"},
		"acme:dev":   {"kind": "tenant"},
		"acme:clean": {"kind": "tenant"},
	})

	tenant, err := environment.GetTenantByName(ctx, "acme")
	require.NoError(t, err)

	mgmtEnv, err := environment.GetByName(ctx, tenant.ID, "mgmt")
	require.NoError(t, err)

	devEnv, err := environment.GetByName(ctx, tenant.ID, "dev")
	require.NoError(t, err)

	setEnvVal := func(envID uuid.UUID, key string, val any, secret bool) {
		t.Helper()
		b, err := json.Marshal(val)
		require.NoError(t, err)
		require.NoError(t, environment.SetEnvironmentValue(ctx, envID, key, b, secret))
	}

	setEnvVal(mgmtEnv.ID, "mgmt_only", "from-mgmt", false)
	setEnvVal(devEnv.ID, "env_only", "from-env", false)
	setEnvVal(devEnv.ID, "secret_token", "s3cr3t", true)

	featureValues := model.Values{
		"computedOnly":                {Computed: &model.Computed{Template: "computed-result"}},
		"globalConfigOnly":            {Config: &model.Config{Type: model.ConfigTypeString}},
		"envConfigOnly":               {Config: &model.Config{Type: model.ConfigTypeString}},
		"globalBeatsChartDefault":     {Config: &model.Config{Type: model.ConfigTypeString}},
		"envBeatsGlobal":              {Config: &model.Config{Type: model.ConfigTypeString}},
		"configBeatsComputed":         {Config: &model.Config{Type: model.ConfigTypeString}, Computed: &model.Computed{Template: "computed-value"}},
		"envConfigBeatsComputed":      {Config: &model.Config{Type: model.ConfigTypeString}, Computed: &model.Computed{Template: "computed-value"}},
		"computedWinsWhenNoConfigSet": {Config: &model.Config{Type: model.ConfigTypeString}, Computed: &model.Computed{Template: "computed-wins"}},
		"computedFromEnv":             {Computed: &model.Computed{Template: `{{ .Env.env_only }}`}},
		"computedFromMgmt":            {Computed: &model.Computed{Template: `{{ .Management.mgmt_only }}`}},
		"computedFromConfig":          {Computed: &model.Computed{Template: `{{ .Configs.globalConfigOnly }}-suffix`}},
		"intConfig":                   {Config: &model.Config{Type: model.ConfigTypeInt}},
		"boolConfig":                  {Config: &model.Config{Type: model.ConfigTypeBool}},
		"computedFromSecretEnv":       {Computed: &model.Computed{Template: `{{ .Env.secret_token }}`}},
		"ignoredForMgmt":              {Config: &model.Config{Type: model.ConfigTypeString}, IgnoreKind: []model.EnvironmentKind{model.EnvironmentKindManagement}},
	}

	chartDefaults := map[string]any{
		"globalBeatsChartDefault": "chart-default",
	}

	mgr.seeder.AddDeploymentWithValues(
		"kitchen-sink", "1.0.0",
		environment.Labels{},
		[]model.EnvironmentKind{model.EnvironmentKindTenant, model.EnvironmentKindManagement},
		featureValues,
		chartDefaults,
		"kitchen sink feature",
	)

	_, err = mgr.seeder.Seed(ctx)
	require.NoError(t, err, "seed deployments")

	createConfig := func(key string, val any, envID *uuid.UUID) {
		t.Helper()
		b, err := json.Marshal(val)
		require.NoError(t, err)
		c := model.NewConfiguration{
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
		require.NoError(t, err, "create config %s", key)
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

	require.NoError(t, deployment.GetManager(ctx).Reconcile(ctx))

	instructions := map[string]message.DeployInstruction{}
	for _, msg := range pub.msg {
		row := mgr.db.pool.QueryRow(ctx,
			`SELECT e.name FROM deploy_instructions di
			 JOIN environments e ON e.id = di.environment_id
			 WHERE di.id = $1`, msg.ID)
		var envName string
		require.NoError(t, row.Scan(&envName))
		instructions[envName] = msg
	}

	require.Contains(t, instructions, "dev", "expected deploy instruction for dev")
	require.Contains(t, instructions, "mgmt", "expected deploy instruction for mgmt")
	require.Contains(t, instructions, "clean", "expected deploy instruction for clean")

	t.Run("dev environment values", func(t *testing.T) {
		vals := instructions["dev"].Values

		assert.NotContains(t, vals, "chartDefault")
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
		require.True(t, ok)
		envMeta, ok := fasit["env"].(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "dev", envMeta["name"])
		assert.Equal(t, "tenant", envMeta["kind"])
		tenantMeta, ok := fasit["tenant"].(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "acme", tenantMeta["name"])
	})

	t.Run("mgmt environment values", func(t *testing.T) {
		vals := instructions["mgmt"].Values

		assert.NotContains(t, vals, "ignoredForMgmt")
		assertValue(t, vals, "globalConfigOnly", "global-val")
		assertValue(t, vals, "globalBeatsChartDefault", "global-val")
		assertValue(t, vals, "intConfig", float64(42))
		assertValue(t, vals, "boolConfig", true)

		assertValue(t, vals, "envBeatsGlobal", "global-val")
		assertValue(t, vals, "configBeatsComputed", "global-val")
		assert.NotContains(t, vals, "envConfigOnly")
		assertValue(t, vals, "envConfigBeatsComputed", "computed-value")
		assertValue(t, vals, "computedWinsWhenNoConfigSet", "computed-wins")
		assertValue(t, vals, "computedOnly", "computed-result")
		assertValue(t, vals, "computedFromEnv", "<no value>")
		assertValue(t, vals, "computedFromMgmt", "from-mgmt")
		assertValue(t, vals, "computedFromConfig", "global-val-suffix")
		assertValue(t, vals, "computedFromSecretEnv", "<no value>")

		fasit, ok := vals["fasit"].(map[string]any)
		require.True(t, ok)
		envMeta, ok := fasit["env"].(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "mgmt", envMeta["name"])
		assert.Equal(t, "management", envMeta["kind"])
	})

	t.Run("clean environment values", func(t *testing.T) {
		vals := instructions["clean"].Values

		assert.NotContains(t, vals, "chartDefault")
		assertValue(t, vals, "ignoredForMgmt", "present")

		assertValue(t, vals, "globalConfigOnly", "global-val")
		assertValue(t, vals, "globalBeatsChartDefault", "global-val")
		assertValue(t, vals, "envBeatsGlobal", "global-val")
		assertValue(t, vals, "configBeatsComputed", "global-val")
		assertValue(t, vals, "intConfig", float64(42))
		assertValue(t, vals, "boolConfig", true)

		assert.NotContains(t, vals, "envConfigOnly")
		assertValue(t, vals, "envConfigBeatsComputed", "computed-value")
		assertValue(t, vals, "computedWinsWhenNoConfigSet", "computed-wins")
		assertValue(t, vals, "computedOnly", "computed-result")
		assertValue(t, vals, "computedFromEnv", "<no value>")
		assertValue(t, vals, "computedFromMgmt", "from-mgmt")
		assertValue(t, vals, "computedFromConfig", "global-val-suffix")
		assertValue(t, vals, "computedFromSecretEnv", "<no value>")

		fasit, ok := vals["fasit"].(map[string]any)
		require.True(t, ok)
		envMeta, ok := fasit["env"].(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "clean", envMeta["name"])
		assert.Equal(t, "tenant", envMeta["kind"])
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
