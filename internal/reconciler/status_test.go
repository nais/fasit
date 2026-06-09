package reconciler

import (
	"testing"

	"github.com/nais/fasit/internal/graph/model"
)

// renderState is what the UI ultimately shows: the derived token, uppercased.
func renderState(disabled bool, deploy model.DeployStatus, action Action) string {
	return NormalizeStatus(deriveState(disabled, deploy, action))
}

// TestDeriveState documents the precedence ladder that resolves the two
// independent logs (decision + deploy) into a single display status. The
// decision log owns pre-deploy state (disabled, unhealthy, failures); the deploy
// log owns the lifecycle of the last sync actually shipped. The two are never
// correlated per event — each case below is named for the rung that decides it.
func TestDeriveState(t *testing.T) {
	const (
		noDeploy   = model.DeployStatusUnknown
		sent       = model.DeployStatusSent
		installing = model.DeployStatusInstalling
		deployed   = model.DeployStatusDeployed
		failed     = model.DeployStatusFailed
	)

	tests := []struct {
		name     string
		disabled bool
		deploy   model.DeployStatus
		action   Action
		want     string
	}{
		// Rung 1: disabled-feature membership wins over everything.
		{"disabled wins over a healthy deploy", true, deployed, ActionSkipUnchanged, "DISABLED"},
		{"disabled wins over an in-flight deploy", true, installing, ActionDeploy, "DISABLED"},
		{"disabled wins over a render failure", true, deployed, ActionFailRender, "DISABLED"},

		// Rung 2: a live deploy lifecycle is shown until it terminates.
		{"deploy sent", false, sent, ActionDeploy, "SENT"},
		{"deploy installing", false, installing, ActionSkipInProgress, "INSTALLING"},
		{"in-flight deploy outranks a concurrent render failure", false, installing, ActionFailRender, "INSTALLING"},
		{"in-flight deploy outranks unhealthy", false, installing, ActionSkipUnhealthy, "INSTALLING"},

		// Rung 3: the reconciler is currently blocked. These must surface even
		// when a previous deploy succeeded — the bug this ladder fixes, since the
		// old code masked any decision once a deploy row existed.
		{"unhealthy surfaces over a stale healthy deploy", false, deployed, ActionSkipUnhealthy, "UNHEALTHY"},
		{"render failure surfaces over a healthy deploy", false, deployed, ActionFailRender, "FAILED"},
		{"missing deps surfaces over a healthy deploy", false, deployed, ActionFailMissingDeps, "FAILED"},
		{"missing config surfaces over a healthy deploy", false, deployed, ActionFailMissingConfig, "FAILED"},

		// Rung 4: with no blocker and no live deploy, the terminal deploy outcome
		// is the truth. Note the decision here is unchanged/deploy — irrelevant.
		{"steady state deployed", false, deployed, ActionSkipUnchanged, "DEPLOYED"},
		{"terminal deploy failure", false, failed, ActionDeploy, "FAILED"},

		// Rung 5: nothing has ever shipped; derive from the decision alone.
		{"first deploy decided, not yet shipped", false, noDeploy, ActionDeploy, "PENDING"},
		{"never deployed, missing deps", false, noDeploy, ActionFailMissingDeps, "FAILED"},
		{"never deployed, unhealthy", false, noDeploy, ActionSkipUnhealthy, "UNHEALTHY"},
		{"never deployed, disabled decision", false, noDeploy, ActionSkipDisabled, "DISABLED"},
		{"never deployed, unknown action", false, noDeploy, Action("bogus"), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderState(tt.disabled, tt.deploy, tt.action); got != tt.want {
				t.Errorf("renderState(disabled=%v, deploy=%q, action=%q) = %q, want %q",
					tt.disabled, tt.deploy, tt.action, got, tt.want)
			}
		})
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct{ input, want string }{
		{"deployed", "DEPLOYED"},
		{"failed", "FAILED"},
		{"pending", "PENDING"},
		{"", "UNKNOWN"},
		{"something", "SOMETHING"},
	}
	for _, tt := range tests {
		if got := NormalizeStatus(tt.input); got != tt.want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
