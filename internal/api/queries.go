package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/model"
)

func (h *HttpHandler) listDeploymentStatuses(ctx context.Context, deploymentID uuid.UUID) (model.DeploymentStatusStates, error) {
	rows, err := h.querier.ListDeploymentStatuses(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment statuses: %w", err)
	}

	states := make(model.DeploymentStatusStates, len(rows))
	for i, status := range rows {
		states[i] = model.DeploymentStatusState(strings.ToUpper(status.Status))
	}

	return states, nil
}
