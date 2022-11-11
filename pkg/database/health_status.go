package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

type HealthRepo interface {
	HealthGet(ctx context.Context, environmentID uuid.UUID) (*model.Health, error)
	HealthStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Health) error
}

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
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.Health{
				ReportedAt: time.Date(1969, 6, 9, 6, 9, 6, 9, time.UTC),
			}, nil
		}
		return nil, err
	}
	return &model.Health{
		EnvironmentID: res.EnvironmentID,
		ReportedAt:    res.ReportedAt,
	}, nil
}
