package model

type DeploymentStatusState string

const (
	DeploymentStatusStateUnknown  DeploymentStatusState = "UNKNOWN"
	DeploymentStatusStateCreated  DeploymentStatusState = "CREATED"
	DeploymentStatusStatePending  DeploymentStatusState = "PENDING"
	DeploymentStatusStateDeployed DeploymentStatusState = "DEPLOYED"
	DeploymentStatusStateFailed   DeploymentStatusState = "FAILED"
	DeploymentStatusStateDisabled DeploymentStatusState = "DISABLED"
)

type DeploymentStatusStates []DeploymentStatusState

func (states DeploymentStatusStates) Aggregate() (state DeploymentStatusState, disabledCount int) {
	if len(states) == 0 {
		return DeploymentStatusStateUnknown, 0
	}

	for _, s := range states {
		if s == DeploymentStatusStateDisabled {
			disabledCount++
		}
	}

	if disabledCount == len(states) {
		return DeploymentStatusStateDisabled, disabledCount
	}

	allDeployed := true
	for _, s := range states {
		if s == DeploymentStatusStateDisabled {
			continue
		}
		switch s {
		case DeploymentStatusStateFailed:
			return DeploymentStatusStateFailed, disabledCount
		case DeploymentStatusStateDeployed:
		default:
			allDeployed = false
		}
	}

	if allDeployed {
		return DeploymentStatusStateDeployed, disabledCount
	}

	return DeploymentStatusStatePending, disabledCount
}

type GitHubCommit struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref"`
}
