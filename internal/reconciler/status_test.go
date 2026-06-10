package reconciler

import (
	"testing"

	"github.com/nais/fasit/internal/feature"
)

// renderState is what the UI ultimately shows: the derived token, uppercased.
func renderState(deploy feature.DeployStatus, action Action) string {
	return NormalizeStatus(deriveState(deploy, action))
}

// TestDeriveState documents the precedence ladder that resolves the two
// independent logs (decision + deploy) into a single display status. The
// decision log owns pre-deploy state (disabled, unhealthy, failures); the deploy
// log owns the lifecycle of the last sync actually shipped. The two are never
// correlated per event — each case below is named for the rung that decides it.
func TestDeriveState(t *testing.T) {
	const (
		noDeploy   = feature.DeployStatusUnknown
		sent       = feature.DeployStatusSent
		installing = feature.DeployStatusInstalling
		deployed   = feature.DeployStatusDeployed
		failed     = feature.DeployStatusFailed
	)

	tests := []struct {
		name   string
		deploy feature.DeployStatus
		action Action
		want   string
	}{
		// Rung 1: a disabled decision wins over everything, including a prior
		// terminal deploy. Disabled is read from the latest decision
		// (ActionSkipDisabled), not a separate lookup.
		{"disabled wins over a healthy deploy", deployed, ActionSkipDisabled, "DISABLED"},
		{"disabled wins over an in-flight deploy", installing, ActionSkipDisabled, "DISABLED"},
		{"disabled with no deploy yet", noDeploy, ActionSkipDisabled, "DISABLED"},

		// Rung 2: a live deploy lifecycle is shown until it terminates.
		{"deploy sent", sent, ActionDeploy, "SENT"},
		{"deploy installing", installing, ActionSkipInProgress, "INSTALLING"},
		{"in-flight deploy outranks a concurrent render failure", installing, ActionFailRender, "INSTALLING"},
		{"in-flight deploy outranks unhealthy", installing, ActionSkipUnhealthy, "INSTALLING"},

		// Rung 3: the reconciler is currently blocked. These must surface even
		// when a previous deploy succeeded — the bug this ladder fixes, since the
		// old code masked any decision once a deploy row existed.
		{"unhealthy surfaces over a stale healthy deploy", deployed, ActionSkipUnhealthy, "UNHEALTHY"},
		{"render failure surfaces over a healthy deploy", deployed, ActionFailRender, "RENDER-ERROR"},
		{"missing deps surfaces over a healthy deploy", deployed, ActionFailMissingDeps, "MISSING-DEPS"},
		{"missing config surfaces over a healthy deploy", deployed, ActionFailMissingConfig, "MISSING-CONFIG"},

		// Rung 4: with no blocker and no live deploy, the terminal deploy outcome
		// is the truth. Note the decision here is unchanged/deploy — irrelevant.
		{"steady state deployed", deployed, ActionSkipUnchanged, "DEPLOYED"},
		{"terminal deploy failure", failed, ActionDeploy, "FAILED"},

		// Rung 5: nothing has ever shipped; derive from the decision alone.
		{"first deploy decided, not yet shipped", noDeploy, ActionDeploy, "PENDING"},
		{"never deployed, missing deps", noDeploy, ActionFailMissingDeps, "MISSING-DEPS"},
		{"never deployed, unhealthy", noDeploy, ActionSkipUnhealthy, "UNHEALTHY"},
		{"never deployed, unknown action", noDeploy, Action("bogus"), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderState(tt.deploy, tt.action); got != tt.want {
				t.Errorf("renderState(deploy=%q, action=%q) = %q, want %q",
					tt.deploy, tt.action, got, tt.want)
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
