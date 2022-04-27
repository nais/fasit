package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/fasit/pkg/graph/model"
)

func (r *queryResolver) Features(ctx context.Context, kind model.EnvironmentKind) ([]*model.Feature, error) {
	features := []*model.Feature{}
	for _, feature := range r.Resolver.Features.Features {
		if !contains(feature.EnvironmentKinds, kind) {
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

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
func contains[T comparable](s []T, value T) bool {
	for _, f := range s {
		if f == value {
			return true
		}
	}
	return false
}
