package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// Environment is the resolver for the environment field.
func (r *configOverrideResolver) Environment(ctx context.Context, obj *model.ConfigOverride) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

// RolloutSummaries is the resolver for the rolloutSummaries field.
func (r *featureResolver) RolloutSummaries(ctx context.Context, obj *model.Feature) ([]*model.RolloutSummary, error) {
	return r.Repo.RolloutSummariesByFeature(ctx, obj.Name)
}

// Configoverrides is the resolver for the configoverrides field.
func (r *featureResolver) Configoverrides(ctx context.Context, obj *model.Feature) ([]*model.ConfigOverride, error) {
	return r.Repo.ConfigOverridesByFeature(ctx, obj.Name)
}

// Features is the resolver for the features field.
func (r *queryResolver) Features(ctx context.Context, kind *model.EnvironmentKind) ([]*model.Feature, error) {
	features := []*model.Feature{}
	for _, feature := range r.Resolver.Features.Features() {
		if kind != nil && !contains(feature.EnvironmentKinds, *kind) {
			continue
		}

		tmp, err := marshalFeature(feature)
		if err != nil {
			return nil, err
		}
		features = append(features, tmp)
	}
	return features, nil
}

// ConfigOverride returns graphgen.ConfigOverrideResolver implementation.
func (r *Resolver) ConfigOverride() graphgen.ConfigOverrideResolver {
	return &configOverrideResolver{r}
}

// Feature returns graphgen.FeatureResolver implementation.
func (r *Resolver) Feature() graphgen.FeatureResolver { return &featureResolver{r} }

type (
	configOverrideResolver struct{ *Resolver }
	featureResolver        struct{ *Resolver }
)
