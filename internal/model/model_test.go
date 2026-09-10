package model

import "testing"

func TestAggregate(t *testing.T) {
	tests := []struct {
		name         string
		states       FeatureReconcileStatusStates
		wantState    FeatureReconcileStatusState
		wantDisabled int
	}{
		{name: "empty", states: nil, wantState: FeatureReconcileStatusStateUnknown},
		{name: "all disabled", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDisabled, FeatureReconcileStatusStateDisabled}, wantState: FeatureReconcileStatusStateDisabled, wantDisabled: 2},
		{name: "deployed", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDeployed, FeatureReconcileStatusStateDeployed}, wantState: FeatureReconcileStatusStateDeployed},
		{name: "deployed with disabled", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDeployed, FeatureReconcileStatusStateDisabled}, wantState: FeatureReconcileStatusStateDeployed, wantDisabled: 1},
		{name: "failed wins over deployed", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDeployed, FeatureReconcileStatusStateFailed}, wantState: FeatureReconcileStatusStateFailed},
		{name: "pending when rolling out", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDeployed, FeatureReconcileStatusStatePending}, wantState: FeatureReconcileStatusStatePending},
		{name: "pending when unknown", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDeployed, FeatureReconcileStatusStateUnknown}, wantState: FeatureReconcileStatusStatePending},

		{name: "missing config alone", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateMissingConfig}, wantState: FeatureReconcileStatusStateMissingConfig},
		{name: "missing config wins over deployed", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateDeployed, FeatureReconcileStatusStateMissingConfig}, wantState: FeatureReconcileStatusStateMissingConfig},
		{name: "missing config wins over pending", states: FeatureReconcileStatusStates{FeatureReconcileStatusStatePending, FeatureReconcileStatusStateMissingConfig}, wantState: FeatureReconcileStatusStateMissingConfig},
		{name: "missing config with disabled", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateMissingConfig, FeatureReconcileStatusStateDisabled}, wantState: FeatureReconcileStatusStateMissingConfig, wantDisabled: 1},
		{name: "failed wins over blocked", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateMissingConfig, FeatureReconcileStatusStateFailed}, wantState: FeatureReconcileStatusStateFailed},
		{name: "missing deps wins over missing config", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateMissingConfig, FeatureReconcileStatusStateMissingDeps}, wantState: FeatureReconcileStatusStateMissingDeps},
		{name: "render error wins over missing deps", states: FeatureReconcileStatusStates{FeatureReconcileStatusStateMissingDeps, FeatureReconcileStatusStateRenderError}, wantState: FeatureReconcileStatusStateRenderError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, disabledCount := tt.states.Aggregate()
			if state != tt.wantState || disabledCount != tt.wantDisabled {
				t.Errorf("Aggregate() = %v, %d; want %v, %d", state, disabledCount, tt.wantState, tt.wantDisabled)
			}
		})
	}
}
