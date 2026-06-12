package environment

import (
	"bytes"
	"strings"
	"testing"

	environment2 "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/ui/components"
)

func TestParseConfigValue_JSONMode(t *testing.T) {
	t.Parallel()
	t.Run("STRING json mode minifies valid JSON", func(t *testing.T) {
		v, err := components.ParseConfigValue("{\n  \"a\": 1,\n  \"b\": [2, 3]\n}", "STRING", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != `{"a":1,"b":[2,3]}` {
			t.Errorf("got %v, want %v", v, `{"a":1,"b":[2,3]}`)
		}
	})

	t.Run("STRING json mode rejects invalid JSON", func(t *testing.T) {
		_, err := components.ParseConfigValue("not json", "STRING", "json")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("error = %v, want to contain 'invalid JSON'", err)
		}
	})

	t.Run("STRING raw mode accepts arbitrary text", func(t *testing.T) {
		v, err := components.ParseConfigValue("not json at all\nmultiline", "STRING", "raw")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "not json at all\nmultiline" {
			t.Errorf("got %v, want original text", v)
		}
	})

	t.Run("STRING_ARRAY json mode rejects non-array JSON", func(t *testing.T) {
		_, err := components.ParseConfigValue(`{"not":"array"}`, "STRING_ARRAY", "json")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("STRING_ARRAY json mode parses array", func(t *testing.T) {
		v, err := components.ParseConfigValue(`["FOO=bar","BAZ=qux"]`, "STRING_ARRAY", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := v.([]string)
		if !ok {
			t.Fatalf("got type %T, want []string", v)
		}
		want := []string{"FOO=bar", "BAZ=qux"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("STRING_ARRAY raw mode falls back to comma split", func(t *testing.T) {
		v, err := components.ParseConfigValue("a, b ,c", "STRING_ARRAY", "raw")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := v.([]string)
		if !ok {
			t.Fatalf("got type %T, want []string", v)
		}
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestTryPrettyJSON(t *testing.T) {
	t.Parallel()
	t.Run("detects object", func(t *testing.T) {
		out, ok := components.TryPrettyJSON(`{"a":1,"b":2}`)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if !strings.Contains(out, "\n") || !strings.Contains(out, `"a": 1`) {
			t.Errorf("got %q, want pretty-printed JSON", out)
		}
	})

	t.Run("rejects plain string", func(t *testing.T) {
		if _, ok := components.TryPrettyJSON("just text"); ok {
			t.Error("expected ok=false")
		}
	})

	t.Run("rejects empty", func(t *testing.T) {
		if _, ok := components.TryPrettyJSON(""); ok {
			t.Error("expected ok=false")
		}
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		if _, ok := components.TryPrettyJSON(`{"a":}`); ok {
			t.Error("expected ok=false")
		}
	})
}

func TestOverviewTab_MasksSecretComputedValue(t *testing.T) {
	t.Parallel()
	feat := &feature.Feature{
		Name: "f",
		FeatureYAML: feature.FeatureYAML{
			Values: feature.Values{
				"safe":   {Computed: &feature.Computed{Template: `{{ .Env.public | quote }}`}},
				"secret": {Computed: &feature.Computed{Template: `{{ .Env.token | quote }}`}},
			},
		},
	}
	page := &FeaturePage{
		TenantSlug:  "t",
		Environment: &Environment{Environment: &environment2.Environment{Name: "e"}},
		Feature: &FeatureDetail{
			Feature: feat,
			Enabled: true,
			ConfigItems: []FeatureConfigItem{
				{Key: "safe", Value: "public-value", Source: string(feature.ConfigSourceHelm), IsComputed: true},
				{Key: "secret", Value: "real-secret", Source: string(feature.ConfigSourceHelm), IsComputed: true, IsSecret: true},
			},
		},
	}

	var buf bytes.Buffer
	if err := overviewTab(page).Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, "public-value") {
		t.Error("non-secret computed value should render its content")
	}
	if strings.Contains(html, "real-secret") {
		t.Error("secret computed value must never appear in rendered HTML")
	}
	if !strings.Contains(html, "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022") {
		t.Error("secret computed value should render as the masked placeholder")
	}
}

func TestOverviewTab_ConfigurableAndComputedShareOneTable(t *testing.T) {
	t.Parallel()
	feat := &feature.Feature{
		Name: "test-feature",
		FeatureYAML: feature.FeatureYAML{
			Values: feature.Values{
				"replicas":      {DisplayName: "Replicas", Config: &feature.Config{Type: feature.ConfigTypeInt}},
				"clusterDomain": {DisplayName: "Cluster Domain", Computed: &feature.Computed{Template: `"x"`}},
			},
		},
	}

	page := &FeaturePage{
		TenantSlug:  "t",
		Environment: &Environment{Environment: &environment2.Environment{Name: "e"}},
		Feature: &FeatureDetail{
			Feature: feat,
			Enabled: true,
			ConfigItems: []FeatureConfigItem{
				{Key: "replicas", Value: "3", Source: string(feature.ConfigSourceHelm), Type: "INT", IsConfigurable: true},
				{Key: "clusterDomain", Value: "e.t.cloud.nais.io", Source: string(feature.ConfigSourceHelm), IsComputed: true},
			},
		},
	}

	var buf bytes.Buffer
	if err := overviewTab(page).Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	// One table only: a single THead, no separate "Computed" heading.
	if n := strings.Count(html, "<thead"); n != 1 {
		t.Errorf("want a single table header, got %d", n)
	}
	if strings.Contains(html, "<h3>Computed</h3>") {
		t.Error("computed items should no longer get their own section heading")
	}

	// Computed rows are still present but carry the visual-cue class so they
	// remain distinguishable from configurable rows.
	if !strings.Contains(html, `id="config-replicas"`) {
		t.Error("configurable item should render")
	}
	if !strings.Contains(html, `class="config-row-computed"`) {
		t.Error("computed item should render with the config-row-computed cue class")
	}

	if !strings.Contains(html, `>values.yaml<`) {
		t.Error("configurable item without override should be tagged 'values.yaml'")
	}
	if !strings.Contains(html, `>computed<`) {
		t.Error("computed item should be tagged 'computed'")
	}
	if !strings.Contains(html, `data-no-sort`) {
		t.Error("actions column should be marked non-sortable")
	}
}

func TestSourceBadge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item FeatureConfigItem
		// want is the rendered source label text.
		want string
		// wantEnvEmphasis asserts whether the "set in this environment" styling applies.
		wantEnvEmphasis bool
	}{
		{
			name:            "environment override is emphasised",
			item:            FeatureConfigItem{Source: string(feature.ConfigSourceEnv), Value: "v"},
			want:            "env config",
			wantEnvEmphasis: true,
		},
		{
			name: "global config row",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceGlobal), Value: "v"},
			want: "global config",
		},
		{
			name: "chart default with a value",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceHelm), Value: "warn"},
			want: "values.yaml",
		},
		{
			name: "unset key, empty value",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceHelm), Value: ""},
			want: "none",
		},
		{
			name: "unset key, empty array",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceHelm), Value: "[]"},
			want: "none",
		},
		{
			name: "computed default is overridable",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceHelm), Value: "x", IsComputed: true, IsConfigurable: true},
			want: "computed default",
		},
		{
			name: "pure computed cannot be overridden",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceHelm), Value: "x", IsComputed: true},
			want: "computed",
		},
		{
			name: "computed takes precedence over empty value",
			item: FeatureConfigItem{Source: string(feature.ConfigSourceHelm), Value: "", IsComputed: true},
			want: "computed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := sourceBadge(tc.item).Render(&buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, ">"+tc.want+"<") {
				t.Errorf("label = %q, want it to contain %q", html, tc.want)
			}
			if got := strings.Contains(html, "source-env-set"); got != tc.wantEnvEmphasis {
				t.Errorf("env emphasis = %v, want %v (%q)", got, tc.wantEnvEmphasis, html)
			}
		})
	}
}

func TestSourceCell_ClearOnlyForEnvOverride(t *testing.T) {
	t.Parallel()
	page := &FeaturePage{
		TenantSlug:  "t",
		Environment: &Environment{Environment: &environment2.Environment{Name: "e"}},
		Feature:     &FeatureDetail{Feature: &feature.Feature{Name: "f"}},
	}
	envItem := FeatureConfigItem{Key: "k", Source: string(feature.ConfigSourceEnv), Value: "v", ID: "00000000-0000-0000-0000-000000000001"}
	helmItem := FeatureConfigItem{Key: "k", Source: string(feature.ConfigSourceHelm), Value: "v"}

	var envBuf, helmBuf bytes.Buffer
	if err := sourceCell(page, envItem).Render(&envBuf); err != nil {
		t.Fatalf("render env: %v", err)
	}
	if err := sourceCell(page, helmItem).Render(&helmBuf); err != nil {
		t.Fatalf("render helm: %v", err)
	}
	if !strings.Contains(envBuf.String(), ">Clear<") {
		t.Error("env override should render an inline Clear control")
	}
	if strings.Contains(helmBuf.String(), ">Clear<") {
		t.Error("non-override row must not render a Clear control")
	}
}
