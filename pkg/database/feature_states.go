package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeatureStateRepo interface {
	FeatureStateGet(ctx context.Context, envID uuid.UUID, featureName string) (*model.FeatureState, error)
	FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *model.Feature, enabled bool) (*model.FeatureState, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	FeatureStatesListen(ctx context.Context, fn ListenFunc) error
	RolloutStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
}

func featureStateFromSQL(state gensql.FeatureState) *model.FeatureState {
	return &model.FeatureState{
		ID:           model.FeatureStateID(state.EnvironmentID, state.Feature),
		EnvID:        state.EnvironmentID,
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
			ID:           model.FeatureStateID(envID, featureState.Name),
			FeatureName:  featureState.Name,
			EnabledAt:    nullTimeToPtr(featureState.EnabledAt),
			Enabled:      featureState.Enabled,
			Created:      featureState.Created.Time,
			LastModified: featureState.LastModified.Time,
			EnvID:        envID,
		})
	}

	// TODO: remove
	exists := map[string]struct{}{}
	for _, f := range ret {
		if f.LastModified.After(time.Now().Add(-5 * 365 * 24 * time.Hour)) {
			exists[f.FeatureName] = struct{}{}
		}
	}

	fsOld, err := r.querier.FeatureStatesGetOld(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ret, nil
		}
		return nil, err
	}

	lookup := map[string]gensql.FeatureStatesGetOldRow{}
	if len(fsOld) > 0 {
		for _, f := range fsOld {
			lookup[f.Feature] = f
		}
	}

	for _, f := range r.oldFeatures.Features() {
		if _, ok := exists[f.Name]; !ok {
			ret = append(ret, &model.FeatureState{
				ID:           model.FeatureStateID(envID, f.Name),
				FeatureName:  f.Name,
				Enabled:      lookup[f.Name].Enabled,
				Created:      lookup[f.Name].Created,
				EnabledAt:    nullTimeToPtr(lookup[f.Name].EnabledAt),
				LastModified: lookup[f.Name].LastModified,
				EnvID:        envID,
			})
		}
	}

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].FeatureName < ret[j].FeatureName
	})
	// TODO: end remove

	return ret, nil
}

func (r *repo) RolloutStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error) {
	ret := []*model.FeatureState{}
	featureStates, err := r.querier.RolloutStatesGet(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ret, nil
		}
		return nil, err
	}

	for _, featureState := range featureStates {
		ret = append(ret, &model.FeatureState{
			ID:           model.FeatureStateID(envID, featureState.FeatureName),
			FeatureName:  featureState.FeatureName,
			EnabledAt:    nullTimeToPtr(featureState.EnabledAt),
			Enabled:      featureState.Enabled,
			Created:      featureState.Created.Time,
			LastModified: featureState.LastModified.Time,
			EnvID:        envID,
		})
	}
	return ret, nil
}

func (r *repo) FeatureStateGet(ctx context.Context, envID uuid.UUID, featureName string) (*model.FeatureState, error) {
	featureState, err := r.querier.FeatureStateGet(ctx, gensql.FeatureStateGetParams{
		EnvironmentID: envID,
		Feature:       featureName,
	})

	if err == nil {
		fs := featureStateFromSQL(featureState)
		fs.EnvID = envID
		return fs, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	fs := &model.FeatureState{
		ID:          model.FeatureStateID(envID, featureName),
		FeatureName: featureName,
		EnvID:       envID,
		Enabled:     false,
	}

	env, err := r.EnvironmentGet(ctx, envID)
	if err != nil {
		return nil, err
	}

	defaultFeatures, err := r.AutoInstallsForKind(ctx, env.Kind)
	if err != nil {
		return nil, err
	}

	for _, feature := range defaultFeatures {
		if feature == featureName {
			fs.Enabled = true
			return fs, nil
		}
	}

	return fs, nil
}

func (r *repo) FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *model.Feature, enabled bool) (*model.FeatureState, error) {
	if len(feature.Dependencies) > 0 {
		states, err := r.FeatureStatesGet(ctx, envID)
		if err != nil {
			return nil, err
		}

		enabledFeatures := []string{}
		for _, state := range states {
			if state.Enabled {
				enabledFeatures = append(enabledFeatures, state.FeatureName)
			}
		}

		fmt.Printf("enabled features: %v\n", enabledFeatures)
		missingFeatures := feature.Dependencies.FindMissing(enabledFeatures)
		if len(missingFeatures) > 0 {
			return nil, fmt.Errorf("dependency '%v' not enabled", missingFeatures)
		}
	}

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
