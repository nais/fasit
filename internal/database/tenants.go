package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type TenantRepo interface {
	TenantGet(ctx context.Context, id uuid.UUID) (*model.Tenant, error)
	TenantGetByName(ctx context.Context, name string) (*model.Tenant, error)
	TenantsGet(ctx context.Context) ([]*model.Tenant, error)
	TenantSetUpgradeDelayDays(ctx context.Context, id uuid.UUID, delayDays int32) (*model.Tenant, error)
}

func tenantFromSQL(t gensql.Tenant) *model.Tenant {
	return &model.Tenant{
		ID:               t.ID,
		Name:             t.Name,
		Description:      nullStringToPtr(t.Description),
		Created:          t.Created.Time,
		LastModified:     t.LastModified.Time,
		UpgradeDelayDays: t.UpgradeDelayDays,
	}
}

func (r *repo) TenantGet(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	tenant, err := r.querier.TenantGet(ctx, id)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func (r *repo) TenantGetByName(ctx context.Context, name string) (*model.Tenant, error) {
	tenant, err := r.querier.TenantGetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func (r *repo) TenantsGet(ctx context.Context) ([]*model.Tenant, error) {
	tenants, err := r.querier.TenantsGet(ctx)
	if err != nil {
		return nil, err
	}
	tenantSlice := []*model.Tenant{}
	for _, tenant := range tenants {
		tenantSlice = append(tenantSlice, tenantFromSQL(tenant))
	}
	return tenantSlice, nil
}

func (r *repo) TenantSetUpgradeDelayDays(ctx context.Context, id uuid.UUID, delayDays int32) (*model.Tenant, error) {
	tenant, err := r.querier.TenantSetUpgradeDelayDays(ctx, gensql.TenantSetUpgradeDelayDaysParams{
		ID:               id,
		UpgradeDelayDays: delayDays,
	})
	if err != nil {
		return nil, err
	}

	r.createAudit(ctx, "updated upgrade_delay_days", "tenants", tenant.ID.String())

	return tenantFromSQL(tenant), nil
}
