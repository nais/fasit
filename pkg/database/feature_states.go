package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

func featureStateFromSQL(state gensql.FeatureState) *model.FeatureState {
	return &model.FeatureState{
		FeatureName:  state.Feature,
		EnabledAt:    nullTimeToPtr(state.EnabledAt),
		Enabled:      state.Enabled,
		Created:      state.Created,
		LastModified: state.LastModified,
	}
}

func (r *repo) FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error) {
	ret := []*model.FeatureState{}
	featureStates, err := r.querier.FeatureStatesGet(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ret, nil
		}
		return nil, err
	}

	for _, featureState := range featureStates {
		ret = append(ret, featureStateFromSQL(featureState))
	}
	return ret, nil
}

func (r *repo) FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *feature.Feature, enabled bool) (*model.FeatureState, error) {
	if len(feature.DependsOn) > 0 {
		states, err := r.querier.FeatureStatesGet(ctx, envID)
		if err != nil {
			return nil, err
		}
		for _, d := range feature.DependsOn {
			for _, fs := range states {
				if fs.Feature == d && !fs.Enabled {
					return nil, fmt.Errorf("dependency '%s' not enabled", d)
				}
			}
		}
	}

	res, err := r.querier.FeatureStateCreateOrUpdate(ctx, gensql.FeatureStateCreateOrUpdateParams{
		EnvironmentID: envID,
		Feature:       feature.Name,
		Enabled:       enabled,
		Enabledat: sql.NullTime{
			Time:  time.Now(),
			Valid: enabled,
		},
	})
	if err != nil {
		return nil, err
	}
	return featureStateFromSQL(res), nil
}
