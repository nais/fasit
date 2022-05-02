package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *mutationResolver) TenantCreate(ctx context.Context, tenant model.TenantCreate) (*model.Tenant, error) {
	return r.Repo.TenantCreate(ctx, &tenant)
}

func (r *queryResolver) Tenants(ctx context.Context) ([]*model.Tenant, error) {
	return r.Repo.TenantsGet(ctx)
}

func (r *queryResolver) Tenant(ctx context.Context, id *uuid.UUID, slug *string) (*model.Tenant, error) {
	if id != nil {
		return r.Repo.TenantGet(ctx, *id)
	}
	if slug != nil {
		return r.Repo.TenantGetByName(ctx, *slug)
	}
	return nil, fmt.Errorf("either ID or slug must be specified")
}

// Mutation returns graphgen.MutationResolver implementation.
func (r *Resolver) Mutation() graphgen.MutationResolver { return &mutationResolver{r} }

// Query returns graphgen.QueryResolver implementation.
func (r *Resolver) Query() graphgen.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
