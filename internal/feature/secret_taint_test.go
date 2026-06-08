package feature

import (
	"encoding/json"
	"testing"

	"github.com/nais/fasit/internal/environment"
)

// renderBoth renders the given feature values twice: once with the supplied
// "real" inputs (controlMV), and once with secret env-values replaced by the
// probe sentinel and secret config values replaced with the probe sentinel.
// It then returns the taint set produced by computedSecretTaint, mirroring
// what HelmValuesWithSecretTaint does without needing a database.
func renderBoth(t *testing.T, values Values, kind environment.EnvironmentKind, controlMV, probeMV *ComputedValues, configs map[string]json.RawMessage, secretConfigKeys []string) map[string]bool {
	t.Helper()

	rawToMap := func(in map[string]json.RawMessage) map[string]any {
		m := map[string]any{}
		for k, v := range in {
			m[k] = json.RawMessage(v)
		}
		return m
	}

	control := rawToMap(configs)
	if err := GenerateWith(values, kind, controlMV, control, deterministicTemplateFuncs); err != nil {
		t.Fatalf("control render: %v", err)
	}

	probe := rawToMap(configs)
	for _, key := range secretConfigKeys {
		setNestedSentinel(probe, key)
	}
	if err := GenerateWith(values, kind, probeMV, probe, deterministicTemplateFuncs); err != nil {
		t.Fatalf("probe render: %v", err)
	}

	return computedSecretTaint(values, control, probe)
}

func TestComputedSecretTaint(t *testing.T) {
	t.Parallel()

	baseTenant := ComputedTenant{Name: "tenant1"}

	tests := []struct {
		name             string
		values           Values
		realMV           *ComputedValues
		probeMV          *ComputedValues
		configs          map[string]json.RawMessage
		secretConfigKeys []string
		wantSecret       []string
		wantNotSecret    []string
	}{
		{
			name: "env value secret used via .Env",
			values: Values{
				"out":  {Computed: &Computed{Template: `{{ .Env.token | quote }}`}},
				"safe": {Computed: &Computed{Template: `{{ .Env.name | quote }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ .Env.token | b64enc }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ with .Env }}{{ .token | quote }}{{ end }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ range .Envs }}{{ .token | quote }}{{ end }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ mapOf "name" "token" .Envs | toJSON }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ if .Env.token }}static{{ end }}`}},
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
			values: Values{
				"out":  {Computed: &Computed{Template: `{{ .Management.token | quote }}`}},
				"safe": {Computed: &Computed{Template: `{{ .Management.public | quote }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ range $k, $v := .Envs }}{{ $v.token | quote }}{{ end }}`}},
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
			values: Values{
				"out": {Computed: &Computed{Template: `{{ eachOf .Envs "token" | toJSON }}`}},
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
			values: Values{
				"plain": {Config: &Config{Type: ConfigTypeString, Secret: true}},
				"out":   {Computed: &Computed{Template: `{{ .Configs.plain | quote }}`}},
				"other": {Config: &Config{Type: ConfigTypeString}},
				"safe":  {Computed: &Computed{Template: `{{ .Configs.other | quote }}`}},
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
			values: Values{
				"plain": {Config: &Config{Type: ConfigTypeString, Secret: true}},
				"out":   {Computed: &Computed{Template: `{{ index .Configs "plain" | quote }}`}},
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
			got := renderBoth(t, tc.values, environment.EnvironmentKindTenant, tc.realMV, tc.probeMV, tc.configs, tc.secretConfigKeys)
			for _, k := range tc.wantSecret {
				if !got[k] {
					t.Errorf("expected key %q to be tainted as secret; taint=%v", k, got)
				}
			}
			for _, k := range tc.wantNotSecret {
				if got[k] {
					t.Errorf("expected key %q not to be tainted as secret; taint=%v", k, got)
				}
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
		want := json.RawMessage(`"` + probeSecretSentinel + `"`)
		if got := m["a"]; string(got.(json.RawMessage)) != string(want) {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("replaces nested leaf", func(t *testing.T) {
		m := map[string]any{
			"outer": map[string]any{
				"inner": json.RawMessage(`"original"`),
			},
		}
		setNestedSentinel(m, "outer.inner")
		inner := m["outer"].(map[string]any)["inner"]
		want := json.RawMessage(`"` + probeSecretSentinel + `"`)
		if string(inner.(json.RawMessage)) != string(want) {
			t.Errorf("got %s, want %s", inner, want)
		}
	})

	t.Run("missing key is a no-op", func(t *testing.T) {
		m := map[string]any{"other": json.RawMessage(`"x"`)}
		setNestedSentinel(m, "missing")
		if string(m["other"].(json.RawMessage)) != `"x"` {
			t.Errorf("map mutated unexpectedly: %v", m)
		}
	})
}

func TestMaskEnvSecrets(t *testing.T) {
	t.Parallel()

	t.Run("masks secret keys in Env, Management, and Envs", func(t *testing.T) {
		mv := &ComputedValues{
			Tenant:     ComputedTenant{Name: "t"},
			Env:        map[string]any{"name": "e1", "token": "real-secret"},
			Management: map[string]any{"name": "mgmt", "token": "mgmt-secret"},
			Envs: []map[string]any{
				{"name": "e1", "token": "real-secret"},
				{"name": "e2", "token": "other-secret"},
			},
		}
		secretKeys := map[string]bool{"token": true}
		maskEnvSecrets(mv, secretKeys)

		if mv.Env["token"] != probeSecretSentinel {
			t.Errorf("Env[token] = %v, want sentinel", mv.Env["token"])
		}
		if mv.Env["name"] != "e1" {
			t.Errorf("Env[name] = %v, want e1", mv.Env["name"])
		}
		if mv.Management["token"] != probeSecretSentinel {
			t.Errorf("Management[token] = %v, want sentinel", mv.Management["token"])
		}
		if mv.Management["name"] != "mgmt" {
			t.Errorf("Management[name] = %v, want mgmt", mv.Management["name"])
		}
		for i, e := range mv.Envs {
			if e["token"] != probeSecretSentinel {
				t.Errorf("Envs[%d][token] = %v, want sentinel", i, e["token"])
			}
		}
	})
}

func TestDeterministicFuncs_NoFalsePositive(t *testing.T) {
	t.Parallel()

	values := Values{
		"out": {Computed: &Computed{Template: `{{ now | date "2006-01-02" }}`}},
	}
	mv := &ComputedValues{
		Tenant: ComputedTenant{Name: "t"},
		Env:    map[string]any{"name": "e1"},
	}

	control := map[string]any{}
	if err := GenerateWith(values, environment.EnvironmentKindTenant, mv, control, deterministicTemplateFuncs); err != nil {
		t.Fatalf("control render: %v", err)
	}

	probe := map[string]any{}
	if err := GenerateWith(values, environment.EnvironmentKindTenant, mv, probe, deterministicTemplateFuncs); err != nil {
		t.Fatalf("probe render: %v", err)
	}

	taint := computedSecretTaint(values, control, probe)
	if len(taint) != 0 {
		t.Errorf("deterministic funcs should produce identical output; got taint=%v", taint)
	}
}

func TestNonStringSecretConfig_ProbeFailsPessimistic(t *testing.T) {
	t.Parallel()

	// A non-string (int) secret config gets a string sentinel, which causes
	// a type mismatch in templates that expect a number. The probe render
	// should fail, and the caller should pessimistically mask all computed
	// values.
	values := Values{
		"port": {Config: &Config{Type: ConfigTypeInt, Secret: true}},
		"out":  {Computed: &Computed{Template: `port={{ .Configs.port }}`}},
	}

	mv := &ComputedValues{
		Tenant: ComputedTenant{Name: "t"},
		Env:    map[string]any{"name": "e1"},
	}

	// Control render with real int value succeeds.
	controlCfg := map[string]any{"port": json.RawMessage(`8080`)}
	if err := GenerateWith(values, environment.EnvironmentKindTenant, mv, controlCfg, deterministicTemplateFuncs); err != nil {
		t.Fatalf("control render: %v", err)
	}

	// Probe render with string sentinel: the template may still succeed
	// (Go templates are loosely typed), but the output will differ from
	// control, so the value is correctly tainted.
	probeCfg := map[string]any{"port": json.RawMessage(`8080`)}
	setNestedSentinel(probeCfg, "port")
	probeErr := GenerateWith(values, environment.EnvironmentKindTenant, mv, probeCfg, deterministicTemplateFuncs)

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
	if !taint["out"] {
		t.Error("non-string secret config should taint computed values that reference it")
	}
}
