package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *repo) HealthGet(ctx context.Context, environmentID uuid.UUID) (*model.Health, error) {
	res, err := r.querier.HealthStatusGet(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	return &model.Health{
		EnvironmentID: res.EnvironmentID,
		ReportedAt:    res.ReportedAt,
	}, nil
}
