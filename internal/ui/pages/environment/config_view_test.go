package environment

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigValue_JSONMode(t *testing.T) {
	t.Parallel()
	t.Run("STRING json mode minifies valid JSON", func(t *testing.T) {
		v, err := parseConfigValue("{\n  \"a\": 1,\n  \"b\": [2, 3]\n}", "STRING", "json")
		require.NoError(t, err)
		assert.Equal(t, `{"a":1,"b":[2,3]}`, v)
	})

	t.Run("STRING json mode rejects invalid JSON", func(t *testing.T) {
		_, err := parseConfigValue("not json", "STRING", "json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("STRING raw mode accepts arbitrary text", func(t *testing.T) {
		v, err := parseConfigValue("not json at all\nmultiline", "STRING", "raw")
		require.NoError(t, err)
		assert.Equal(t, "not json at all\nmultiline", v)
	})

	t.Run("STRING_ARRAY json mode rejects non-array JSON", func(t *testing.T) {
		_, err := parseConfigValue(`{"not":"array"}`, "STRING_ARRAY", "json")
		require.Error(t, err)
	})

	t.Run("STRING_ARRAY json mode parses array", func(t *testing.T) {
		v, err := parseConfigValue(`["FOO=bar","BAZ=qux"]`, "STRING_ARRAY", "json")
		require.NoError(t, err)
		assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, v)
	})

	t.Run("STRING_ARRAY raw mode falls back to comma split", func(t *testing.T) {
		v, err := parseConfigValue("a, b ,c", "STRING_ARRAY", "raw")
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, v)
	})
}

func TestTryPrettyJSON(t *testing.T) {
	t.Parallel()
	t.Run("detects object", func(t *testing.T) {
		out, ok := tryPrettyJSON(`{"a":1,"b":2}`)
		require.True(t, ok)
		assert.Contains(t, out, "\n")
		assert.Contains(t, out, `"a": 1`)
	})

	t.Run("rejects plain string", func(t *testing.T) {
		_, ok := tryPrettyJSON("just text")
		assert.False(t, ok)
	})

	t.Run("rejects empty", func(t *testing.T) {
		_, ok := tryPrettyJSON("")
		assert.False(t, ok)
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		_, ok := tryPrettyJSON(`{"a":}`)
		assert.False(t, ok)
	})
}

func TestOverviewTab_SplitsConfigurableAndComputed(t *testing.T) {
	t.Parallel()
	feat := &model.Feature{
		Name: "test-feature",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"replicas":      {DisplayName: "Replicas", Config: &model.Config{Type: model.ConfigTypeInt}},
				"clusterDomain": {DisplayName: "Cluster Domain", Computed: &model.Computed{Template: `"x"`}},
			},
		},
	}

	page := &FeaturePage{
		TenantSlug:  "t",
		Environment: &Environment{Environment: &model.Environment{Name: "e"}},
		Feature: &FeatureDetail{
			Feature: feat,
			Enabled: true,
			ConfigItems: []FeatureConfigItem{
				{Key: "replicas", Value: "3", Source: string(model.ConfigSourceHelm), Type: "INT", IsConfigurable: true},
				{Key: "clusterDomain", Value: "e.t.cloud.nais.io", Source: string(model.ConfigSourceHelm), IsComputed: true},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, overviewTab(page).Render(&buf))
	html := buf.String()

	configIdx := strings.Index(html, "Configuration")
	computedIdx := strings.Index(html, "Computed")
	require.NotEqual(t, -1, configIdx, "Configuration heading should be present")
	require.NotEqual(t, -1, computedIdx, "Computed heading should be present")
	assert.Less(t, configIdx, computedIdx, "Configuration should render before Computed")

	// Source pills tag each row with where its value came from.
	assert.Contains(t, html, `source-label">default`, "configurable item without override should be tagged default")
	assert.Contains(t, html, `source-label">mapping`, "computed item should be tagged mapping")
	// Actions column should be marked non-sortable.
	assert.Contains(t, html, `data-no-sort`)
}

func TestOverviewTab_SourceLabels(t *testing.T) {
	t.Parallel()
	feat := &model.Feature{
		Name: "f",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"a": {Config: &model.Config{Type: model.ConfigTypeString}},
				"b": {Config: &model.Config{Type: model.ConfigTypeString}},
			},
		},
	}
	page := &FeaturePage{
		TenantSlug:  "t",
		Environment: &Environment{Environment: &model.Environment{Name: "e"}},
		Feature: &FeatureDetail{
			Feature: feat,
			Enabled: true,
			ConfigItems: []FeatureConfigItem{
				{Key: "a", Value: "x", Source: string(model.ConfigSourceHelm), Type: "STRING", IsConfigurable: true},
				{Key: "b", Value: "y", Source: string(model.ConfigSourceEnv), Type: "STRING", IsConfigurable: true, ID: "00000000-0000-0000-0000-000000000001"},
			},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, overviewTab(page).Render(&buf))
	html := buf.String()
	// Override row → "config" pill; non-override row → "default" pill.
	assert.Contains(t, html, `>config<`, "env-override item should display 'config' source pill")
	assert.Contains(t, html, `>default<`, "non-overridden item should display 'default' source pill")
}
