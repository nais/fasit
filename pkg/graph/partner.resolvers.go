package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *mutationResolver) PartnerCreate(ctx context.Context, partner model.PartnerCreate) (*model.Partner, error) {
	return r.Repo.PartnerCreate(ctx, &partner)
}

func (r *queryResolver) Partners(ctx context.Context) ([]*model.Partner, error) {
	return r.Repo.PartnersGet(ctx)
}

// Mutation returns graphgen.MutationResolver implementation.
func (r *Resolver) Mutation() graphgen.MutationResolver { return &mutationResolver{r} }

// Query returns graphgen.QueryResolver implementation.
func (r *Resolver) Query() graphgen.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
