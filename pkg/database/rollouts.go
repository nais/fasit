package database

import (
	"context"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type RolloutRepo interface {
	RolloutCreate(ctx context.Context, name, chart, version string) (*model.Rollout, error)
}

func (r *repo) RolloutCreate(ctx context.Context, name, chart, version string) (*model.Rollout, error) {
	ro, err := r.querier.RolloutCreate(ctx, gensql.RolloutCreateParams{FeatureName: name, Chart: chart, Version: version})
	if err != nil {
		return nil, err
	}

	return &model.Rollout{
		ID:          ro.ID,
		Version:     ro.Version,
		Chart:       ro.Chart,
		Created:     ro.Created,
		FeatureName: ro.FeatureName,
	}, nil
}
