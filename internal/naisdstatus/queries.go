package naisdstatus

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatussql"
)

func Get(ctx context.Context, environmentID uuid.UUID) (*model.Health, error) {
	res, err := querier(ctx).GetNaisdHealthStatus(ctx, environmentID)
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

func Set(ctx context.Context, environmentID uuid.UUID, h *message.Health) error {
	_, err := querier(ctx).SetNaisdHealthStatus(ctx, naisdstatussql.SetNaisdHealthStatusParams{
		EnvironmentID: environmentID,
		ReportedAt:    h.ReportedAt,
	})

	return err
}

func ListReleaseStatuses(ctx context.Context, environmentID uuid.UUID) ([]*model.Release, error) {
	res, err := querier(ctx).ListReleaseStatuses(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	releases := make([]*model.Release, len(res))
	for i, r := range res {
		releases[i] = &model.Release{
			Name:         r.Feature,
			Version:      r.Version,
			Status:       r.Status,
			Revision:     int(r.Revision),
			LastDeployed: r.LastDeployed,
			Created:      r.Created,
			LastModified: r.LastModified,
		}
	}

	return releases, nil
}
