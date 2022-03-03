package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/c3po/pkg/graph/model"
)

func (r *mutationResolver) ConfigurationCreate(ctx context.Context, configuration model.NewConfiguration) (*model.Configuration, error) {
	return r.Repo.ConfigCreate(ctx, configuration)
}

func (r *queryResolver) Configuration(ctx context.Context, feature string) (*model.Configuration, error) {
	return r.Repo.ConfigGet(ctx, feature)
}
