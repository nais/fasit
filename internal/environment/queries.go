package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

// QuerierKey is exposed so tests can inject fake queriers on the context.
// Avoid usage by e.g. using testcontainers.
const QuerierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, QuerierKey, environmentsql.New(pool))
}

func querier(ctx context.Context) environmentsql.Querier {
	return ctx.Value(QuerierKey).(environmentsql.Querier)
}

func Create(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error) {
	env, err := querier(ctx).Create(ctx, environmentsql.CreateParams{
		Name:        t.Name,
		Description: t.Description,
		TenantID:    t.TenantID,
		Kind:        types.EnvironmentKind(t.Kind),
	})
	if err != nil {
		return nil, err
	}

	audit.CreateAudit(ctx, "created", "environments", env.ID.String())

	return environmentFromSQL(env), nil
}

func SetLabels(ctx context.Context, environmentID uuid.UUID, labels Labels) error {
	lbls := make(types.EnvironmentLabels)
	maps.Copy(lbls, labels)

	return querier(ctx).SetLabels(ctx, environmentsql.SetLabelsParams{
		Labels: lbls,
		ID:     environmentID,
	})
}

func SetEnvironmentValue(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage, secret bool) error {
	err := querier(ctx).SetEnvironmentValue(ctx, environmentsql.SetEnvironmentValueParams{
		Envid:  environmentID,
		Key:    key,
		Value:  value,
		Secret: secret,
	})
	if err != nil {
		return fmt.Errorf("failed to store environment value: %w", err)
	}

	audit.CreateAudit(ctx, "created or updated", "environment_values", environmentID.String()+":"+key)

	return nil
}

func TenantEnvironments(ctx context.Context, onlyReconciled bool) ([]*model.TenantEnvironment, error) {
	data, err := querier(ctx).TenantEnvironments(ctx, !onlyReconciled)
	if err != nil {
		return nil, err
	}

	var ret []*model.TenantEnvironment
	for _, d := range data {
		ret = append(ret, &model.TenantEnvironment{
			Environment: model.Environment{
				ID:           d.ID,
				Name:         d.Name,
				Description:  d.Description,
				Created:      d.Created.Time,
				LastModified: d.LastModified.Time,
				Kind:         model.EnvironmentKind(d.Kind),
			},
			TenantName: d.TenantName,
			TenantID:   d.TenantID,
		})
	}

	return ret, nil
}

func Get(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	env, err := querier(ctx).Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func GetTenant(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	tenant, err := querier(ctx).GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func GetTenants(ctx context.Context) ([]*model.Tenant, error) {
	tenants, err := querier(ctx).GetTenants(ctx)
	if err != nil {
		return nil, err
	}
	tenantSlice := []*model.Tenant{}
	for _, tenant := range tenants {
		tenantSlice = append(tenantSlice, tenantFromSQL(tenant))
	}
	return tenantSlice, nil
}

func GetTenantGetByName(ctx context.Context, name string) (*model.Tenant, error) {
	tenant, err := querier(ctx).GetTenantByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func CreateTenant(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error) {
	tenant, err := querier(ctx).TenantCreate(ctx, environmentsql.TenantCreateParams{
		Name:        t.Name,
		Description: t.Description,
	})
	if err != nil {
		return nil, err
	}

	audit.CreateAudit(ctx, "created", "tenants", tenant.ID.String())

	return tenantFromSQL(tenant), nil
}
