package deployment

func AggregateState(statuses []*DeploymentStatus) (state DeploymentStatusState, disabledCount int) {
	nonNil := make([]*DeploymentStatus, 0, len(statuses))
	for _, s := range statuses {
		if s != nil {
			nonNil = append(nonNil, s)
		}
	}

	if len(nonNil) == 0 {
		return DeploymentStatusStateUnknown, 0
	}

	for _, s := range nonNil {
		if s.State == DeploymentStatusStateDisabled {
			disabledCount++
		}
	}

	if disabledCount == len(nonNil) {
		return DeploymentStatusStateDisabled, disabledCount
	}

	allDeployed := true
	for _, s := range nonNil {
		if s.State == DeploymentStatusStateDisabled {
			continue
		}
		switch s.State {
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
