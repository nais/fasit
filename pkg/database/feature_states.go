package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeatureStateRepo interface {
	FeatureStateGet(ctx context.Context, envID uuid.UUID, featureName string) (*model.FeatureState, error)
	FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *feature.Feature, enabled bool) (*model.FeatureState, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	FeatureStatesListen(ctx context.Context, fn ListenFunc) error
}

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
		ret = append(ret, &model.FeatureState{
			FeatureName:   featureState.Feature,
			EnabledAt:     nullTimeToPtr(featureState.EnabledAt),
			Enabled:       featureState.Enabled,
			Created:       featureState.Created,
			LastModified:  featureState.LastModified,
			RolloutStatus: model.RolloutStatus(featureState.RolloutStatus),
			EnvID:         envID,
		})
	}
	return ret, nil
}

func (r *repo) FeatureStateGet(ctx context.Context, envID uuid.UUID, featureName string) (*model.FeatureState, error) {
	featureState, err := r.querier.FeatureStateGet(ctx, gensql.FeatureStateGetParams{
		EnvironmentID: envID,
		Feature:       featureName,
	})
	if err != nil {
		return nil, err
	}
	fs := featureStateFromSQL(featureState)
	fs.EnvID = envID
	return fs, nil
}

func (r *repo) FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *feature.Feature, enabled bool) (*model.FeatureState, error) {
	// if len(feature.DependsOn) > 0 {
	// 	states, err := r.querier.FeatureStatesGet(ctx, envID)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	enabledFeatures := []string{}
	// 	for _, state := range states {
	// 		if state.Enabled {
	// 			enabledFeatures = append(enabledFeatures, state.Feature)
	// 		}
	// 	}

	// 	missingFeatures := feature.DependsOn.FindMissing(enabledFeatures)
	// 	if len(missingFeatures) > 0 {
	// 		return nil, fmt.Errorf("dependency '%v' not enebled", missingFeatures)
	// 	}
	// }

	res, err := r.querier.FeatureStateCreateOrUpdate(ctx, gensql.FeatureStateCreateOrUpdateParams{
		EnvironmentID: envID,
		Feature:       feature.Name,
		Enabled:       enabled,
		Enabledat: sql.NullTime{
			Time:  Now(ctx),
			Valid: enabled,
		},
	})
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("enabled %v", feature.Name)
	if !enabled {
		msg = fmt.Sprintf("disabled %v", feature.Name)
	}

	r.createAudit(ctx, msg, "feature_states", envID.String()+":"+feature.Name)

	return featureStateFromSQL(res), nil
}

func (r *repo) FeatureStatesListen(ctx context.Context, fn ListenFunc) error {
	return r.ListenNotify(ctx, "feature_states_notify", fn)
}
