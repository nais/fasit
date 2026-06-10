package reconciler

import (
	"testing"

	"github.com/nais/fasit/internal/feature"
)

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
		name       string
		deploy     feature.DeployStatus
		action     Action
		want       string
		wantSource stateSource
	}{
		// Rung 1: a disabled decision wins over everything, including a prior
		// terminal deploy. Disabled is read from the latest decision
		// (ActionSkipDisabled), not a separate lookup, and owns the timestamp.
		{"disabled wins over a healthy deploy", deployed, ActionSkipDisabled, "DISABLED", sourceDecision},
		{"disabled wins over an in-flight deploy", installing, ActionSkipDisabled, "DISABLED", sourceDecision},
		{"disabled with no deploy yet", noDeploy, ActionSkipDisabled, "DISABLED", sourceDecision},

		// Rung 2: a live deploy lifecycle is shown until it terminates.
		{"deploy sent", sent, ActionDeploy, "SENT", sourceDeploy},
		{"deploy installing", installing, ActionSkipInProgress, "INSTALLING", sourceDeploy},
		{"in-flight deploy outranks a concurrent render failure", installing, ActionFailRender, "INSTALLING", sourceDeploy},
		{"in-flight deploy outranks unhealthy", installing, ActionSkipUnhealthy, "INSTALLING", sourceDeploy},

		// Rung 3: the reconciler is currently blocked. These must surface even
		// when a previous deploy succeeded — the bug this ladder fixes, since the
		// old code masked any decision once a deploy row existed. The blocking
		// decision owns the timestamp.
		{"unhealthy surfaces over a stale healthy deploy", deployed, ActionSkipUnhealthy, "UNHEALTHY", sourceDecision},
		{"render failure surfaces over a healthy deploy", deployed, ActionFailRender, "RENDER-ERROR", sourceDecision},
		{"missing deps surfaces over a healthy deploy", deployed, ActionFailMissingDeps, "MISSING-DEPS", sourceDecision},
		{"missing config surfaces over a healthy deploy", deployed, ActionFailMissingConfig, "MISSING-CONFIG", sourceDecision},

		// Rung 4: with no blocker and no live deploy, the terminal deploy outcome
		// is the truth and owns the timestamp — so re-enabling (a fresh decision,
		// no new deploy) does not reset the "when" of a steady DEPLOYED status.
		{"steady state deployed", deployed, ActionSkipUnchanged, "DEPLOYED", sourceDeploy},
		{"steady deploy after a re-enable decision", deployed, ActionDeploy, "DEPLOYED", sourceDeploy},
		{"terminal deploy failure", failed, ActionDeploy, "FAILED", sourceDeploy},

		// Rung 5: nothing has ever shipped; derive from the decision alone, which
		// owns the timestamp.
		{"first deploy decided, not yet shipped", noDeploy, ActionDeploy, "PENDING", sourceDecision},
		{"never deployed, missing deps", noDeploy, ActionFailMissingDeps, "MISSING-DEPS", sourceDecision},
		{"never deployed, unhealthy", noDeploy, ActionSkipUnhealthy, "UNHEALTHY", sourceDecision},
		{"never deployed, unknown action", noDeploy, Action("bogus"), "UNKNOWN", sourceDecision},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, src := deriveState(tt.deploy, tt.action)
			if got := NormalizeStatus(state); got != tt.want {
				t.Errorf("deriveState(deploy=%q, action=%q) state = %q, want %q",
					tt.deploy, tt.action, got, tt.want)
			}
			if src != tt.wantSource {
				t.Errorf("deriveState(deploy=%q, action=%q) source = %v, want %v",
					tt.deploy, tt.action, src, tt.wantSource)
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
