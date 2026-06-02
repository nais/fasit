package feature

import (
	"testing"

	"github.com/nais/fasit/internal/graph/model"
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

	data := &HelmRenderData{
		MV: &ComputedValues{
			Tenant: ComputedTenant{Name: "dev-nais"},
			Env: map[string]any{
				"name":        "dev",
				"slack_token": "xoxb-dev-token",
			},
		},
		EnvKind:       model.EnvironmentKindTenant,
		ConfigMap:     map[string]any{},
		SecretEnvKeys: map[string]bool{"slack_token": true},
	}

	rendered, taint, probeOK, err := renderHelmValuesWithSecretTaint(data, f)
	if err != nil {
		t.Fatalf("renderHelmValuesWithSecretTaint: %v", err)
	}
	if !probeOK {
		t.Fatal("probe must succeed for this template")
	}

	if got := rendered["slackAlertUrl"]; got != "https://hooks.slack.com/services/xoxb-dev-token/alerts" {
		t.Errorf("slackAlertUrl = %v, want unmasked URL", got)
	}
	if !taint["slackAlertUrl"] {
		t.Error("slackAlertUrl depends on secret env slack_token and must be tainted")
	}
	if taint["notificationUrl"] {
		t.Error("notificationUrl uses no secret and must not be tainted")
	}
}
