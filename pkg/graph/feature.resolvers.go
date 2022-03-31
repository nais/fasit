package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/fasit/pkg/graph/model"
)

func (r *queryResolver) Features(ctx context.Context) ([]*model.Feature, error) {
	features := []*model.Feature{}
	for _, feature := range r.Resolver.Features.Features {
		tmp, err := marshalFeature(feature)
		if err != nil {
			return nil, err
		}
		features = append(features, tmp)
	}
	return features, nil
}
