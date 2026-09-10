package model

type FeatureReconcileStatusState string

const (
	FeatureReconcileStatusStateUnknown  FeatureReconcileStatusState = "UNKNOWN"
	FeatureReconcileStatusStatePending  FeatureReconcileStatusState = "PENDING"
	FeatureReconcileStatusStateDeployed FeatureReconcileStatusState = "DEPLOYED"
	FeatureReconcileStatusStateFailed   FeatureReconcileStatusState = "FAILED"
	FeatureReconcileStatusStateDisabled FeatureReconcileStatusState = "DISABLED"

	FeatureReconcileStatusStateRenderError   FeatureReconcileStatusState = "RENDER-ERROR"
	FeatureReconcileStatusStateMissingDeps   FeatureReconcileStatusState = "MISSING-DEPS"
	FeatureReconcileStatusStateMissingConfig FeatureReconcileStatusState = "MISSING-CONFIG"
)

// blockedPrecedence orders the pre-deploy blocked states by precedence when an
// assignment is blocked in several environments for different reasons. All of
// them mean "needs human attention before rollout can proceed", so any single
// one is enough to surface in aggregated views.
var blockedPrecedence = []FeatureReconcileStatusState{
	FeatureReconcileStatusStateRenderError,
	FeatureReconcileStatusStateMissingDeps,
	FeatureReconcileStatusStateMissingConfig,
}

type FeatureReconcileStatusStates []FeatureReconcileStatusState

func (states FeatureReconcileStatusStates) Aggregate() (state FeatureReconcileStatusState, disabledCount int) {
	if len(states) == 0 {
		return FeatureReconcileStatusStateUnknown, 0
	}

	for _, s := range states {
		if s == FeatureReconcileStatusStateDisabled {
			disabledCount++
		}
	}

	if disabledCount == len(states) {
		return FeatureReconcileStatusStateDisabled, disabledCount
	}

	blocked := map[FeatureReconcileStatusState]bool{}
	allDeployed := true
	for _, s := range states {
		if s == FeatureReconcileStatusStateDisabled {
			continue
		}
		switch s {
		case FeatureReconcileStatusStateFailed:
			return FeatureReconcileStatusStateFailed, disabledCount
		case FeatureReconcileStatusStateRenderError, FeatureReconcileStatusStateMissingDeps, FeatureReconcileStatusStateMissingConfig:
			blocked[s] = true
			allDeployed = false
		case FeatureReconcileStatusStateDeployed:
		default:
			allDeployed = false
		}
	}

	// A blocked environment means the rollout cannot proceed without human
	// input (config, dependencies, or a template fix). Surface that over
	// DEPLOYED/PENDING so the friction is visible rather than looking like an
	// ordinary in-flight rollout.
	for _, s := range blockedPrecedence {
		if blocked[s] {
			return s, disabledCount
		}
	}

	if allDeployed {
		return FeatureReconcileStatusStateDeployed, disabledCount
	}

	return FeatureReconcileStatusStatePending, disabledCount
}

type GitHubCommit struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref"`
}
