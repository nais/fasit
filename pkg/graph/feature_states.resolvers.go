package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *featureStateResolver) Feature(ctx context.Context, featureState *model.FeatureState) (*model.Feature, error) {
	feature := r.Resolver.Features.Get(featureState.FeatureName)
	if feature == nil {
		return nil, nil
	}

	return marshalFeature(*feature)
}

// FeatureState returns graphgen.FeatureStateResolver implementation.
func (r *Resolver) FeatureState() graphgen.FeatureStateResolver { return &featureStateResolver{r} }

type featureStateResolver struct{ *Resolver }
