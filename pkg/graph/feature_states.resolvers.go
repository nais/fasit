package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// Feature is the resolver for the feature field.
func (r *featureStateResolver) Feature(ctx context.Context, obj *model.FeatureState) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// FeatureStateSave is the resolver for the featureStateSave field.
func (r *mutationResolver) FeatureStateSave(ctx context.Context, envID uuid.UUID, enabled bool, feature string) (*model.FeatureState, error) {
	feat := r.Resolver.Features.Get(feature)
	if feat == nil {
		return nil, nil
	}
	return r.Repo.FeatureStatesCreateOrUpdate(ctx, envID, feat, enabled)
}

// FeatureState returns graphgen.FeatureStateResolver implementation.
func (r *Resolver) FeatureState() graphgen.FeatureStateResolver { return &featureStateResolver{r} }

type featureStateResolver struct{ *Resolver }
