package database

import (
	"context"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type RolloutRepo interface {
	RolloutCreate(ctx context.Context, name, chart, version string) (*model.Rollout, error)
	RolloutsListen(ctx context.Context, fn ListenFunc) error
}

func (r *repo) RolloutCreate(ctx context.Context, name, chart, version string) (*model.Rollout, error) {
	ro, err := r.querier.RolloutCreate(ctx, gensql.RolloutCreateParams{FeatureName: name, Version: version})
	if err != nil {
		return nil, err
	}

	return &model.Rollout{
		ID:          ro.ID,
		Version:     ro.Version,
		Created:     ro.Created,
		FeatureName: ro.FeatureName,
	}, nil
}

func (r *repo) RolloutsListen(ctx context.Context, fn ListenFunc) error {
	return r.ListenNotify(ctx, "rollout_notify", fn)
}
