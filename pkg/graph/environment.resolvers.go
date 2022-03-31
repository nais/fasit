package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *environmentResolver) FeatureStates(ctx context.Context, environment *model.Environment) ([]*model.FeatureState, error) {
	return r.Repo.FeatureStatesGet(ctx, environment.ID)
}

func (r *mutationResolver) EnvironmentCreate(ctx context.Context, environment model.EnvironmentCreate) (*model.Environment, error) {
	return r.Repo.EnvironmentCreate(ctx, &environment)
}

func (r *mutationResolver) EnvironmentUpdate(ctx context.Context, id uuid.UUID, input model.EnvironmentUpdate) (*model.Environment, error) {
	return r.Repo.EnvironmentUpdate(ctx, id, &input)
}

func (r *queryResolver) Environment(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, id)
}

func (r *queryResolver) Environments(ctx context.Context, partnerID uuid.UUID) ([]*model.Environment, error) {
	return r.Repo.EnvironmentsGet(ctx, partnerID)
}

// Environment returns graphgen.EnvironmentResolver implementation.
func (r *Resolver) Environment() graphgen.EnvironmentResolver { return &environmentResolver{r} }

type environmentResolver struct{ *Resolver }
