package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

func featureStateFromSQL(state gensql.FeatureState) *model.FeatureState {
	return &model.FeatureState{
		FeatureName:  state.Feature,
		Enabled:      state.Enabled,
		Created:      state.Created,
		LastModified: state.LastModified,
	}
}

func (r *Repo) FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error) {
	featureStates, err := r.querier.FeatureStatesGet(ctx, envID)
	if err != nil {
		return nil, err
	}

	ret := []*model.FeatureState{}
	for _, featureState := range featureStates {
		ret = append(ret, featureStateFromSQL(featureState))
	}
	return ret, nil
}
