package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (r *queryResolver) Values(ctx context.Context, feature string, env uuid.UUID) (map[string]interface{}, error) {
	f := r.Resolver.Features.Get(feature)

	if f == nil {
		return nil, fmt.Errorf("feature %s not found", feature)
	}
	return r.Repo.HelmValues(ctx, *f, env, f.RequiredFields())
}
