package feature

import (
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderHelmValuesWithSecretTaint_NoLeakBetweenRenders is a regression
// test for the bug where the real render mutated the shared configMap by
// writing its computed values back into it, and control/probe then cloned
// that already-mutated map. Because addToMap is write-once, control/probe
// skipped re-rendering any computed key, the diff was always empty, and
// no computed value ever got tainted as secret.
func TestRenderHelmValuesWithSecretTaint_NoLeakBetweenRenders(t *testing.T) {
	t.Parallel()

	f := &model.Feature{
		Name: "dependencytrack",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"slackAlertUrl": model.Value{
					Computed: &model.Computed{Template: `"https://hooks.slack.com/services/{{ .Env.slack_token }}/alerts"`},
				},
				"notificationUrl": model.Value{
					Computed: &model.Computed{Template: `"https://dt.{{ .Env.name }}.example.com/notify"`},
				},
			},
		},
	}

	data := &helmRenderData{
		mv: &ComputedValues{
			Tenant: ComputedTenant{Name: "dev-nais"},
			Env: map[string]any{
				"name":        "dev",
				"slack_token": "xoxb-dev-token",
			},
		},
		envKind:       model.EnvironmentKindTenant,
		configMap:     map[string]any{},
		secretEnvKeys: map[string]bool{"slack_token": true},
	}

	rendered, taint, probeOK, err := renderHelmValuesWithSecretTaint(data, f)
	require.NoError(t, err)
	require.True(t, probeOK, "probe must succeed for this template")

	assert.Equal(t, "https://hooks.slack.com/services/xoxb-dev-token/alerts", rendered["slackAlertUrl"],
		"real render should produce the unmasked URL")
	assert.True(t, taint["slackAlertUrl"],
		"slackAlertUrl depends on secret env slack_token and must be tainted")
	assert.False(t, taint["notificationUrl"],
		"notificationUrl uses no secret and must not be tainted")
}
