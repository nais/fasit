package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// RolloutSummaries is the resolver for the rolloutSummaries field.
func (r *featureResolver) RolloutSummaries(ctx context.Context, obj *model.Feature) ([]*model.RolloutSummary, error) {
	return r.Repo.RolloutSummariesByFeature(ctx, obj.Name)
}

// Features is the resolver for the features field.
func (r *queryResolver) Features(ctx context.Context, kind *model.EnvironmentKind) ([]*model.Feature, error) {
	features := []*model.Feature{}
	for _, feature := range r.Resolver.Features.Features {
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

// Feature returns graphgen.FeatureResolver implementation.
func (r *Resolver) Feature() graphgen.FeatureResolver { return &featureResolver{r} }

type featureResolver struct{ *Resolver }
