package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/model"
)

func (h *HttpHandler) listReconcileStatuses(ctx context.Context, featureAssignmentID uuid.UUID) (model.FeatureReconcileStatusStates, error) {
	rows, err := h.querier.ListReconcileStatuses(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("get feature assignment statuses: %w", err)
	}

	states := make(model.FeatureReconcileStatusStates, len(rows))
	for i, status := range rows {
		states[i] = model.FeatureReconcileStatusState(strings.ToUpper(status.Status))
	}

	return states, nil
}
