package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/c3po/pkg/graph/model"
)

func (r *mutationResolver) EnvironmentCreate(ctx context.Context, environment model.EnvironmentCreate) (*model.Environment, error) {
	return r.Repo.EnvironmentCreate(ctx, &environment)
}
