package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/model"
)

func (h *HttpHandler) listReconcileStatuses(ctx context.Context, featureAssignmentID uuid.UUID) (model.FeatureReconcileStatusStates, error) {
	rows, err := h.querier.ListReconcileSignals(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("get feature assignment statuses: %w", err)
	}

	states := make(model.FeatureReconcileStatusStates, len(rows))
	for i, row := range rows {
		state := featureassignment.DeriveReconcileState(featureassignment.ReconcileSignals{
			DeployStatus:   row.DeployStatus,
			DecisionAction: row.DecisionAction,
			Disabled:       row.Disabled,
		})
		states[i] = model.FeatureReconcileStatusState(strings.ToUpper(state))
	}

	return states, nil
}
