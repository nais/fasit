package feature

import (
	"encoding/json"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderBoth renders the given feature values twice: once with the supplied
// "real" inputs (controlMV), and once with secret env-values replaced by the
// probe sentinel and secret config values replaced with the probe sentinel.
// It then returns the taint set produced by computedSecretTaint, mirroring
// what HelmValuesWithSecretTaint does without needing a database.
func renderBoth(t *testing.T, values model.Values, kind model.EnvironmentKind, controlMV, probeMV *ComputedValues, configs map[string]json.RawMessage, secretConfigKeys []string) map[string]bool {
	t.Helper()

	rawToMap := func(in map[string]json.RawMessage) map[string]any {
		m := map[string]any{}
		for k, v := range in {
			m[k] = json.RawMessage(v)
		}
		return m
	}

	control := rawToMap(configs)
	require.NoError(t, GenerateWith(values, kind, controlMV, control, deterministicTemplateFuncs))

	probe := rawToMap(configs)
	for _, key := range secretConfigKeys {
		setNestedSentinel(probe, key)
	}
	require.NoError(t, GenerateWith(values, kind, probeMV, probe, deterministicTemplateFuncs))

	return computedSecretTaint(values, control, probe)
}

func TestComputedSecretTaint(t *testing.T) {
	t.Parallel()

	baseTenant := ComputedTenant{Name: "tenant1"}

	tests := []struct {
		name             string
		values           model.Values
		realMV           *ComputedValues
		probeMV          *ComputedValues
		configs          map[string]json.RawMessage
		secretConfigKeys []string
		wantSecret       []string
		wantNotSecret    []string
	}{
		{
			name: "env value secret used via .Env",
			values: model.Values{
				"out":  {Computed: &model.Computed{Template: `{{ .Env.token | quote }}`}},
				"safe": {Computed: &model.Computed{Template: `{{ .Env.name | quote }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"name": "env1", "token": "real-secret"},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"name": "env1", "token": probeSecretSentinel},
			},
			wantSecret:    []string{"out"},
			wantNotSecret: []string{"safe"},
		},
		{
			name: "secret hidden behind b64enc still tainted",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ .Env.token | b64enc }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"token": "real-secret"},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"token": probeSecretSentinel},
			},
			wantSecret: []string{"out"},
		},
		{
			name: "with-block dot rebinding caught by render diff",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ with .Env }}{{ .token | quote }}{{ end }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"token": "real-secret"},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"token": probeSecretSentinel},
			},
			wantSecret: []string{"out"},
		},
		{
			name: "range over .Envs with implicit dot",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ range .Envs }}{{ .token | quote }}{{ end }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Envs:   []map[string]any{{"token": "real-secret"}},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Envs:   []map[string]any{{"token": probeSecretSentinel}},
			},
			wantSecret: []string{"out"},
		},
		{
			name: "mapOf helper with string key arg",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ mapOf "name" "token" .Envs | toJSON }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Envs: []map[string]any{
					{"name": "e1", "token": "real-secret"},
				},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Envs: []map[string]any{
					{"name": "e1", "token": probeSecretSentinel},
				},
			},
			wantSecret: []string{"out"},
		},
		{
			name: "secret referenced but discarded is not tainted",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ if .Env.token }}static{{ end }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"token": "real-secret"},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Env:    map[string]any{"token": probeSecretSentinel},
			},
			wantNotSecret: []string{"out"},
		},
		{
			name: "management value secret used via .Management",
			values: model.Values{
				"out":  {Computed: &model.Computed{Template: `{{ .Management.token | quote }}`}},
				"safe": {Computed: &model.Computed{Template: `{{ .Management.public | quote }}`}},
			},
			realMV: &ComputedValues{
				Tenant:     baseTenant,
				Management: map[string]any{"token": "real-mgmt-secret", "public": "mgmt-public"},
			},
			probeMV: &ComputedValues{
				Tenant:     baseTenant,
				Management: map[string]any{"token": probeSecretSentinel, "public": "mgmt-public"},
			},
			wantSecret:    []string{"out"},
			wantNotSecret: []string{"safe"},
		},
		{
			name: "two-variable range over .Envs",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ range $k, $v := .Envs }}{{ $v.token | quote }}{{ end }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Envs:   []map[string]any{{"token": "real-secret"}},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Envs:   []map[string]any{{"token": probeSecretSentinel}},
			},
			wantSecret: []string{"out"},
		},
		{
			name: "eachOf helper with string key arg",
			values: model.Values{
				"out": {Computed: &model.Computed{Template: `{{ eachOf .Envs "token" | toJSON }}`}},
			},
			realMV: &ComputedValues{
				Tenant: baseTenant,
				Envs: []map[string]any{
					{"name": "e1", "token": "real-secret"},
				},
			},
			probeMV: &ComputedValues{
				Tenant: baseTenant,
				Envs: []map[string]any{
					{"name": "e1", "token": probeSecretSentinel},
				},
			},
			wantSecret: []string{"out"},
		},
		{
			name: "secret config value used in computed",
			values: model.Values{
				"plain": {Config: &model.Config{Type: model.ConfigTypeString, Secret: true}},
				"out":   {Computed: &model.Computed{Template: `{{ .Configs.plain | quote }}`}},
				"other": {Config: &model.Config{Type: model.ConfigTypeString}},
				"safe":  {Computed: &model.Computed{Template: `{{ .Configs.other | quote }}`}},
			},
			realMV:  &ComputedValues{Tenant: baseTenant, Env: map[string]any{"name": "env1"}},
			probeMV: &ComputedValues{Tenant: baseTenant, Env: map[string]any{"name": "env1"}},
			configs: map[string]json.RawMessage{
				"plain": json.RawMessage(`"real-secret"`),
				"other": json.RawMessage(`"not-secret"`),
			},
			secretConfigKeys: []string{"plain"},
			wantSecret:       []string{"out"},
			wantNotSecret:    []string{"safe"},
		},
		{
			name: "secret config accessed via index helper still tainted",
			values: model.Values{
				"plain": {Config: &model.Config{Type: model.ConfigTypeString, Secret: true}},
				"out":   {Computed: &model.Computed{Template: `{{ index .Configs "plain" | quote }}`}},
			},
			realMV:  &ComputedValues{Tenant: baseTenant, Env: map[string]any{"name": "env1"}},
			probeMV: &ComputedValues{Tenant: baseTenant, Env: map[string]any{"name": "env1"}},
			configs: map[string]json.RawMessage{
				"plain": json.RawMessage(`"real-secret"`),
			},
			secretConfigKeys: []string{"plain"},
			wantSecret:       []string{"out"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderBoth(t, tc.values, model.EnvironmentKindTenant, tc.realMV, tc.probeMV, tc.configs, tc.secretConfigKeys)
			for _, k := range tc.wantSecret {
				assert.Truef(t, got[k], "expected key %q to be tainted as secret; taint=%v", k, got)
			}
			for _, k := range tc.wantNotSecret {
				assert.Falsef(t, got[k], "expected key %q not to be tainted as secret; taint=%v", k, got)
			}
		})
	}
}

func TestSetNestedSentinel(t *testing.T) {
	t.Parallel()

	t.Run("replaces leaf value", func(t *testing.T) {
		m := map[string]any{
			"a": json.RawMessage(`"original"`),
		}
		setNestedSentinel(m, "a")
		assert.Equal(t, json.RawMessage(`"`+probeSecretSentinel+`"`), m["a"])
	})

	t.Run("replaces nested leaf", func(t *testing.T) {
		m := map[string]any{
			"outer": map[string]any{
				"inner": json.RawMessage(`"original"`),
			},
		}
		setNestedSentinel(m, "outer.inner")
		inner := m["outer"].(map[string]any)["inner"]
		assert.Equal(t, json.RawMessage(`"`+probeSecretSentinel+`"`), inner)
	})

	t.Run("missing key is a no-op", func(t *testing.T) {
		m := map[string]any{"other": json.RawMessage(`"x"`)}
		setNestedSentinel(m, "missing")
		assert.Equal(t, map[string]any{"other": json.RawMessage(`"x"`)}, m)
	})
}

func TestMaskEnvSecrets(t *testing.T) {
	t.Parallel()

	t.Run("masks secret keys in Env, Management, and Envs", func(t *testing.T) {
		mv := &ComputedValues{
			Tenant: ComputedTenant{Name: "t"},
			Env:    map[string]any{"name": "e1", "token": "real-secret"},
			Management: map[string]any{"name": "mgmt", "token": "mgmt-secret"},
			Envs: []map[string]any{
				{"name": "e1", "token": "real-secret"},
				{"name": "e2", "token": "other-secret"},
			},
		}
		secretKeys := map[string]bool{"token": true}
		maskEnvSecrets(mv, secretKeys)

		assert.Equal(t, probeSecretSentinel, mv.Env["token"])
		assert.Equal(t, "e1", mv.Env["name"])
		assert.Equal(t, probeSecretSentinel, mv.Management["token"])
		assert.Equal(t, "mgmt", mv.Management["name"])
		for _, e := range mv.Envs {
			assert.Equal(t, probeSecretSentinel, e["token"])
		}
	})
}

func TestDeterministicFuncs_NoFalsePositive(t *testing.T) {
	t.Parallel()

	values := model.Values{
		"out": {Computed: &model.Computed{Template: `{{ now | date "2006-01-02" }}`}},
	}
	mv := &ComputedValues{
		Tenant: ComputedTenant{Name: "t"},
		Env:    map[string]any{"name": "e1"},
	}

	control := map[string]any{}
	require.NoError(t, GenerateWith(values, model.EnvironmentKindTenant, mv, control, deterministicTemplateFuncs))

	probe := map[string]any{}
	require.NoError(t, GenerateWith(values, model.EnvironmentKindTenant, mv, probe, deterministicTemplateFuncs))

	taint := computedSecretTaint(values, control, probe)
	assert.Empty(t, taint, "deterministic funcs should produce identical output; no false positive taint")
}

func TestNonStringSecretConfig_ProbeFailsPessimistic(t *testing.T) {
	t.Parallel()

	// A non-string (int) secret config gets a string sentinel, which causes
	// a type mismatch in templates that expect a number. The probe render
	// should fail, and the caller should pessimistically mask all computed
	// values.
	values := model.Values{
		"port": {Config: &model.Config{Type: model.ConfigTypeInt, Secret: true}},
		"out":  {Computed: &model.Computed{Template: `port={{ .Configs.port }}`}},
	}

	mv := &ComputedValues{
		Tenant: ComputedTenant{Name: "t"},
		Env:    map[string]any{"name": "e1"},
	}

	// Control render with real int value succeeds.
	controlCfg := map[string]any{"port": json.RawMessage(`8080`)}
	require.NoError(t, GenerateWith(values, model.EnvironmentKindTenant, mv, controlCfg, deterministicTemplateFuncs))

	// Probe render with string sentinel: the template may still succeed
	// (Go templates are loosely typed), but the output will differ from
	// control, so the value is correctly tainted.
	probeCfg := map[string]any{"port": json.RawMessage(`8080`)}
	setNestedSentinel(probeCfg, "port")
	probeErr := GenerateWith(values, model.EnvironmentKindTenant, mv, probeCfg, deterministicTemplateFuncs)

	if probeErr != nil {
		// Probe failed — caller should pessimistically mask everything.
		// This is the expected path for templates that do arithmetic on the value.
		t.Log("probe render failed as expected for non-string secret:", probeErr)
		return
	}

	// If the probe happens to succeed (template doesn't do type-specific
	// operations), the taint comparison should still flag the key because
	// the sentinel differs from the real value.
	taint := computedSecretTaint(values, controlCfg, probeCfg)
	assert.True(t, taint["out"], "non-string secret config should taint computed values that reference it")
}
