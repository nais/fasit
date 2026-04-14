package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// TenantCreate is the resolver for the tenantCreate field.
func (r *mutationResolver) TenantCreate(ctx context.Context, tenant model.TenantCreate) (*model.Tenant, error) {
	return environment.CreateTenant(ctx, &tenant)
}

func (r *mutationResolver) TenantSetUpgradeDelayDays(ctx context.Context, tenantID uuid.UUID, delayDays int) (*model.Tenant, error) {
	panic(fmt.Errorf("not implemented: TenantSetUpgradeDelayDays - tenantSetUpgradeDelayDays"))
}

// Tenant is the resolver for the tenant field.
func (r *queryResolver) Tenant(ctx context.Context, id *uuid.UUID, slug *string) (*model.Tenant, error) {
	if id != nil {
		return environment.GetTenant(ctx, *id)
	}
	if slug != nil {
		return environment.GetTenantGetByName(ctx, *slug)
	}
	return nil, fmt.Errorf("either ID or slug must be specified")
}

// Tenants is the resolver for the tenants field.
func (r *queryResolver) Tenants(ctx context.Context) ([]*model.Tenant, error) {
	return environment.GetTenants(ctx)
}

// Environments is the resolver for the environments field.
func (r *tenantResolver) Environments(ctx context.Context, obj *model.Tenant) ([]*model.Environment, error) {
	return r.Repo.EnvironmentsGet(ctx, obj.ID)
}

// Environment is the resolver for the environment field.
func (r *tenantResolver) Environment(ctx context.Context, obj *model.Tenant, id *uuid.UUID, slug *string) (*model.Environment, error) {
	if id != nil {
		return r.Repo.EnvironmentGet(ctx, *id)
	}
	if slug != nil {
		return r.Repo.EnvironmentGetByName(ctx, obj.ID, *slug)
	}
	return nil, fmt.Errorf("either ID or slug must be specified")
}

// Warnings is the resolver for the warnings field.
func (r *tenantResolver) Warnings(ctx context.Context, obj *model.Tenant) ([]model.Warning, error) {
	return environment.Warnings(ctx, nil, &obj.ID)
}

func (r *Resolver) Tenant() graphgen.TenantResolver { return &tenantResolver{r} }

type tenantResolver struct{ *Resolver }
