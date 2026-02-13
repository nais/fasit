package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// TenantCreate is the resolver for the tenantCreate field.
func (r *mutationResolver) TenantCreate(ctx context.Context, tenant model.TenantCreate) (*model.Tenant, error) {
	return r.Repo.TenantCreate(ctx, &tenant)
}

// TenantSetUpgradeDelayDays is the resolver for the tenantSetUpgradeDelayDays field.
func (r *mutationResolver) TenantSetUpgradeDelayDays(ctx context.Context, tenantID uuid.UUID, delayDays int) (*model.Tenant, error) {
	delayDays32, err := database.ToInt32(delayDays)
	if err != nil {
		return nil, err
	}
	return r.Repo.TenantSetUpgradeDelayDays(ctx, tenantID, delayDays32)
}

// Tenant is the resolver for the tenant field.
func (r *queryResolver) Tenant(ctx context.Context, id *uuid.UUID, slug *string) (*model.Tenant, error) {
	if id != nil {
		return r.Repo.TenantGet(ctx, *id)
	}
	if slug != nil {
		return r.Repo.TenantGetByName(ctx, *slug)
	}
	return nil, fmt.Errorf("either ID or slug must be specified")
}

// ClusterUpgradeHistory is the resolver for the clusterUpgradeHistory field.
func (r *queryResolver) ClusterUpgradeHistory(ctx context.Context, limit *int, offset *int) (*model.ClusterUpgradeHistoryResult, error) {
	var limitValue, offsetValue int32
	if limit != nil {
		limitValue = int32(*limit) // #nosec G115 -- int is at least 32 bits on all Go platforms, conversion is safe for valid pagination values
	}
	if offset != nil {
		offsetValue = int32(*offset) // #nosec G115 -- int is at least 32 bits on all Go platforms, conversion is safe for valid pagination values
	}

	return r.Repo.ClusterUpgradeHistoryGetAll(ctx, limitValue, offsetValue)
}

// Tenants is the resolver for the tenants field.
func (r *queryResolver) Tenants(ctx context.Context) ([]*model.Tenant, error) {
	return r.Repo.TenantsGet(ctx)
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

// ClusterUpgradeHistory is the resolver for the clusterUpgradeHistory field.
func (r *tenantResolver) ClusterUpgradeHistory(ctx context.Context, obj *model.Tenant, limit *int, offset *int) (*model.ClusterUpgradeHistoryResult, error) {
	var limitValue, offsetValue int32
	if limit != nil {
		limitValue = int32(*limit) // #nosec G115 -- int is at least 32 bits on all Go platforms, conversion is safe for valid pagination values
	}
	if offset != nil {
		offsetValue = int32(*offset) // #nosec G115 -- int is at least 32 bits on all Go platforms, conversion is safe for valid pagination values
	}

	return r.Repo.ClusterUpgradeHistoryGetByTenant(ctx, obj.ID, limitValue, offsetValue)
}

func (r *Resolver) Tenant() graphgen.TenantResolver { return &tenantResolver{r} }

type tenantResolver struct{ *Resolver }
