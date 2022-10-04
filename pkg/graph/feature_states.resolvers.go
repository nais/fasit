package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// Feature is the resolver for the feature field.
func (r *featureStateResolver) Feature(ctx context.Context, obj *model.FeatureState) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// MissingDependencies is the resolver for the missingDependencies field.
func (r *featureStateResolver) MissingDependencies(ctx context.Context, obj *model.FeatureState) ([]*model.Feature, error) {
	f := r.Features.Get(obj.FeatureName)

	states, err := r.Repo.FeatureStatesGet(ctx, obj.EnvID)
	if err != nil {
		return nil, err
	}

	enabledFeatures := []string{}
	for _, s := range states {
		if s.Enabled && s.RolloutStatus == model.RolloutStatusDeployed {
			enabledFeatures = append(enabledFeatures, s.FeatureName)
		}
	}

	ret := []*model.Feature{}

	for _, d := range f.DependsOn.FindMissing(enabledFeatures) {
		feat := r.Features.Get(d)
		if feat == nil {
			return nil, fmt.Errorf("invalid dependency %v", d)
		}
		f, err := marshalFeature(*feat)
		if err != nil {
			return nil, err
		}
		ret = append(ret, f)
	}
	return ret, nil
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
