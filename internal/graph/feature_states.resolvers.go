package graph

import (
	"context"
	"errors"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// Feature is the resolver for the feature field.
func (r *featureStateResolver) Feature(ctx context.Context, obj *model.FeatureState) (*model.Feature, error) {
	f, err := r.Repo.FeatureByNameForEnv(ctx, obj.FeatureName, obj.EnvID)
	if err != nil {
		return nil, err
	}
	f.GraphVars.EnvironmentID = obj.EnvID
	return f, nil
}

// MissingDependencies is the resolver for the missingDependencies field.
func (r *featureStateResolver) MissingDependencies(ctx context.Context, obj *model.FeatureState) ([]*model.Feature, error) {
	return r.missingDependencies(ctx, obj.FeatureName, obj.EnvID)
}

// Configuration is the resolver for the configuration field.
func (r *featureStateResolver) Configuration(ctx context.Context, obj *model.FeatureState) (*model.Configurations, error) {
	return r.Resolver.Query().Configuration(ctx, obj.FeatureName, &obj.EnvID)
}

// FeatureStateSave is the resolver for the featureStateSave field.
func (r *mutationResolver) FeatureStateSave(ctx context.Context, envID uuid.UUID, enabled bool, feature string) (*model.FeatureState, error) {
	feat, err := r.Repo.FeatureByNameForEnv(ctx, feature, envID)
	if err != nil {
		return nil, err
	}
	return r.Repo.FeatureStatesCreateOrUpdate(ctx, envID, feat, enabled)
}

// FeatureState is the resolver for the featureState field.
func (r *queryResolver) FeatureState(ctx context.Context, envID uuid.UUID, feature string) (*model.FeatureState, error) {
	fs, err := r.Repo.FeatureStateGet(ctx, envID, feature)
	if err == nil {
		return fs, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// If no feature state exists, return a default feature state
	fs = &model.FeatureState{
		ID:          model.FeatureStateID(envID, feature),
		FeatureName: feature,
		Enabled:     false,
		EnvID:       envID,
	}
	return fs, nil
}

func (r *Resolver) FeatureState() graphgen.FeatureStateResolver { return &featureStateResolver{r} }

type featureStateResolver struct{ *Resolver }
