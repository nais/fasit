package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/google/uuid"
)

func (r *queryResolver) Values(ctx context.Context, feature string, env uuid.UUID) (map[string]any, error) {
	return r.Repo.HelmValues(ctx, feature, env)
}
