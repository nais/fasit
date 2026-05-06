package deployment

import "testing"

func TestAggregateState(t *testing.T) {
	tests := []struct {
		name         string
		states       []DeploymentStatusState
		wantState    DeploymentStatusState
		wantDisabled int
	}{
		{"no statuses", nil, DeploymentStatusStateUnknown, 0},
		{"empty slice", []DeploymentStatusState{}, DeploymentStatusStateUnknown, 0},
		{"all disabled", []DeploymentStatusState{DeploymentStatusStateDisabled, DeploymentStatusStateDisabled}, DeploymentStatusStateDisabled, 2},
		{"one failed", []DeploymentStatusState{DeploymentStatusStateDeployed, DeploymentStatusStateFailed}, DeploymentStatusStateFailed, 0},
		{"failed wins over disabled", []DeploymentStatusState{DeploymentStatusStateDisabled, DeploymentStatusStateFailed}, DeploymentStatusStateFailed, 1},
		{"failed counts disabled across whole slice", []DeploymentStatusState{DeploymentStatusStateFailed, DeploymentStatusStateDisabled, DeploymentStatusStateDisabled}, DeploymentStatusStateFailed, 2},
		{"all deployed", []DeploymentStatusState{DeploymentStatusStateDeployed, DeploymentStatusStateDeployed}, DeploymentStatusStateDeployed, 0},
		{"deployed plus disabled", []DeploymentStatusState{DeploymentStatusStateDeployed, DeploymentStatusStateDisabled}, DeploymentStatusStateDeployed, 1},
		{"pending mixed", []DeploymentStatusState{DeploymentStatusStateDeployed, DeploymentStatusStatePending}, DeploymentStatusStatePending, 0},
		{"pending mixed with disabled", []DeploymentStatusState{DeploymentStatusStateDisabled, DeploymentStatusStatePending}, DeploymentStatusStatePending, 1},
		{"created counts as pending", []DeploymentStatusState{DeploymentStatusStateCreated, DeploymentStatusStateDeployed}, DeploymentStatusStatePending, 0},
		{"created with disabled counts as pending", []DeploymentStatusState{DeploymentStatusStateDisabled, DeploymentStatusStateCreated}, DeploymentStatusStatePending, 1},
		{"unknown counts as pending", []DeploymentStatusState{DeploymentStatusStateUnknown, DeploymentStatusStateDeployed}, DeploymentStatusStatePending, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statuses := make([]*DeploymentStatus, len(tt.states))
			for i, s := range tt.states {
				statuses[i] = &DeploymentStatus{State: s}
			}
			gotState, gotDisabled := AggregateState(statuses)
			if gotState != tt.wantState {
				t.Errorf("state = %q, want %q", gotState, tt.wantState)
			}
			if gotDisabled != tt.wantDisabled {
				t.Errorf("disabledCount = %d, want %d", gotDisabled, tt.wantDisabled)
			}
		})
	}
}

func TestAggregateState_skipsNilEntries(t *testing.T) {
	statuses := []*DeploymentStatus{
		nil,
		{State: DeploymentStatusStateDeployed},
		nil,
		{State: DeploymentStatusStateDeployed},
	}
	state, disabled := AggregateState(statuses)
	if state != DeploymentStatusStateDeployed {
		t.Errorf("state = %q, want %q", state, DeploymentStatusStateDeployed)
	}
	if disabled != 0 {
		t.Errorf("disabledCount = %d, want 0", disabled)
	}
}
