package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

func (r *repo) HealthStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Health) error {
	_, err := r.querier.HealthStatusCreateOrUpdate(ctx, gensql.HealthStatusCreateOrUpdateParams{
		EnvironmentID: environmentID,
		ReportedAt:    h.ReportedAt,
	})

	return err
}
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
