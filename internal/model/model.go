package model

type FeatureReconcileStatusState string

const (
	FeatureReconcileStatusStateUnknown  FeatureReconcileStatusState = "UNKNOWN"
	FeatureReconcileStatusStatePending  FeatureReconcileStatusState = "PENDING"
	FeatureReconcileStatusStateDeployed FeatureReconcileStatusState = "DEPLOYED"
	FeatureReconcileStatusStateFailed   FeatureReconcileStatusState = "FAILED"
	FeatureReconcileStatusStateDisabled FeatureReconcileStatusState = "DISABLED"
)

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

	allDeployed := true
	for _, s := range states {
		if s == FeatureReconcileStatusStateDisabled {
			continue
		}
		switch s {
		case FeatureReconcileStatusStateFailed:
			return FeatureReconcileStatusStateFailed, disabledCount
		case FeatureReconcileStatusStateDeployed:
		default:
			allDeployed = false
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
