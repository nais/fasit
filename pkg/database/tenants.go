package database

import (
	"context"

	"github.com/google/uuid"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type TenantRepo interface {
	TenantCI(ctx context.Context) (*model.Tenant, error)
	TenantCreate(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error)
	TenantEnvironments(ctx context.Context) ([]*model.TenantEnvironments, error)
	TenantGet(ctx context.Context, id uuid.UUID) (*model.Tenant, error)
	TenantGetByName(ctx context.Context, name string) (*model.Tenant, error)
	TenantsGet(ctx context.Context) ([]*model.Tenant, error)
}

func tenantFromSQL(t gensql.Tenant) *model.Tenant {
	return &model.Tenant{
		ID:           t.ID,
		Name:         t.Name,
		Description:  nullStringToPtr(t.Description),
		Created:      t.Created,
		LastModified: t.LastModified,
	}
}

func (r *repo) TenantCreate(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error) {
	tenant, err := r.querier.TenantCreate(ctx, gensql.TenantCreateParams{
		Name:        t.Name,
		Description: ptrToNullString(t.Description),
	})
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
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

func (r *repo) TenantEnvironments(ctx context.Context) ([]*model.TenantEnvironments, error) {
	data, err := r.querier.TenantEnvironments(ctx)
	if err != nil {
		return nil, err
	}

	var ret []*model.TenantEnvironments
	for _, d := range data {
		ret = append(ret, &model.TenantEnvironments{
			Environment: model.Environment{
				ID:           d.ID,
				Name:         d.Name,
				Description:  nullStringToPtr(d.Description),
				Created:      d.Created,
				LastModified: d.LastModified,
				Kind:         model.EnvironmentKind(d.Kind),
			},
			TenantName: d.TenantName,
			TenantID:   d.TenantID,
		})
	}

	return ret, nil
}

func (r *repo) TenantCI(ctx context.Context) (*model.Tenant, error) {
	tenant, err := r.querier.TenantCI(ctx)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}
