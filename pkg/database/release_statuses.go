package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

type ReleaseStatusRepo interface {
	ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error
	ReleaseStatusesGet(ctx context.Context, environmentID uuid.UUID) ([]*model.Release, error)
}

func (r *repo) ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error {
	_, err := r.querier.ReleaseStatusCreateOrUpdate(ctx, gensql.ReleaseStatusCreateOrUpdateParams{
		EnvironmentID: environmentID,
		Feature:       h.Name,
		Version:       h.Version,
		Status:        h.Status,
		Revision:      int32(h.Revision),
		LastDeployed:  h.LastDeployed,
	})

	return err
}

func (r *repo) ReleaseStatusesGet(ctx context.Context, environmentID uuid.UUID) ([]*model.Release, error) {
	res, err := r.querier.ReleaseStatusesGet(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	releases := make([]*model.Release, len(res))
	for i, r := range res {
		releases[i] = &model.Release{
			FeatureName:  r.Feature,
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
