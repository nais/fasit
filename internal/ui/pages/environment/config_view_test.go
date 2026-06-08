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

func TestOverviewTab_SplitsConfigurableAndComputed(t *testing.T) {
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

	configIdx := strings.Index(html, "Configuration")
	computedIdx := strings.Index(html, "Computed")
	if configIdx == -1 {
		t.Fatal("Configuration heading should be present")
	}
	if computedIdx == -1 {
		t.Fatal("Computed heading should be present")
	}
	if configIdx >= computedIdx {
		t.Error("Configuration should render before Computed")
	}

	if !strings.Contains(html, `source-label">helm value`) {
		t.Error("configurable item without override should be tagged 'helm value'")
	}
	if !strings.Contains(html, `source-label">mapping`) {
		t.Error("computed item should be tagged 'mapping'")
	}
	if !strings.Contains(html, `data-no-sort`) {
		t.Error("actions column should be marked non-sortable")
	}
}

func TestOverviewTab_SourceLabels(t *testing.T) {
	t.Parallel()
	feat := &feature.Feature{
		Name: "f",
		FeatureYAML: feature.FeatureYAML{
			Values: feature.Values{
				"a": {Config: &feature.Config{Type: feature.ConfigTypeString}},
				"b": {Config: &feature.Config{Type: feature.ConfigTypeString}},
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
				{Key: "a", Value: "x", Source: string(feature.ConfigSourceHelm), Type: "STRING", IsConfigurable: true},
				{Key: "b", Value: "y", Source: string(feature.ConfigSourceEnv), Type: "STRING", IsConfigurable: true, ID: "00000000-0000-0000-0000-000000000001"},
			},
		},
	}
	var buf bytes.Buffer
	if err := overviewTab(page).Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `>env config<`) {
		t.Error("env-override item should display 'env config' source pill")
	}
	if !strings.Contains(html, `>helm value<`) {
		t.Error("non-overridden item should display 'helm value' source pill")
	}
}
