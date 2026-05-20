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
	"github.com/nais/fasit/internal/dbtx"
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
	var env environmentsql.Environment
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		env, err = querier(ctx).Create(ctx, environmentsql.CreateParams{
			Name:        t.Name,
			Description: t.Description,
			TenantID:    t.TenantID,
			Kind:        types.EnvironmentKind(t.Kind),
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Description: "created",
			ObjectType:  "environments",
			ObjectID:    env.ID.String(),
		})
	})
	if err != nil {
		return nil, err
	}

	tenant, err := GetTenant(ctx, t.TenantID)
	if err != nil {
		return nil, fmt.Errorf("get tenant for labels: %w", err)
	}

	lbls := Labels{"name": t.Name, "tenant": tenant.Name}
	for k, v := range t.Labels {
		lbls[k] = v
	}
	if err := SetLabels(ctx, env.ID, lbls); err != nil {
		return nil, fmt.Errorf("set labels: %w", err)
	}

	return environmentFromSQL(env), nil
}

func SetLabels(ctx context.Context, environmentID uuid.UUID, labels Labels) error {
	existing, err := querier(ctx).GetLabels(ctx, environmentID)
	if err != nil {
		existing = nil
	}

	lbls := make(types.EnvironmentLabels)
	maps.Copy(lbls, existing)
	maps.Copy(lbls, labels)

	return querier(ctx).SetLabels(ctx, environmentsql.SetLabelsParams{
		Labels: lbls,
		ID:     environmentID,
	})
}

func GetLabels(ctx context.Context, environmentID uuid.UUID) (Labels, error) {
	labels, err := querier(ctx).GetLabels(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	lbls := make(Labels)
	maps.Copy(lbls, labels)

	return lbls, nil
}

func SetEnvironmentValue(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage, secret bool) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err := querier(ctx).SetEnvironmentValue(ctx, environmentsql.SetEnvironmentValueParams{
			EnvironmentID: environmentID,
			Key:           key,
			Value:         value,
			Secret:        secret,
		})
		if err != nil {
			return fmt.Errorf("failed to store environment value: %w", err)
		}

		return audit.Create(ctx, audit.CreateParams{
			Description: "created or updated",
			ObjectType:  "environment_values",
			Metadata:    environmentID.String() + ":" + key,
		})
	})
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

func GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*model.Environment, error) {
	env, err := querier(ctx).GetByName(ctx, environmentsql.GetByNameParams{
		TenantID: tenantID,
		Name:     name,
	})
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

func GetTenantByName(ctx context.Context, name string) (*model.Tenant, error) {
	tenant, err := querier(ctx).GetTenantByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func CreateTenant(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error) {
	var tenant environmentsql.Tenant
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		tenant, err = querier(ctx).TenantCreate(ctx, environmentsql.TenantCreateParams{
			Name:        t.Name,
			Description: t.Description,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Description: "created",
			ObjectType:  "tenants",
			Metadata:    tenant.ID.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func List(ctx context.Context, tenantID uuid.UUID) ([]*model.Environment, error) {
	envs, err := querier(ctx).List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	environmentSlice := make([]*model.Environment, len(envs))
	for i, env := range envs {
		environmentSlice[i] = environmentFromSQL(env)
	}
	return environmentSlice, nil
}

func GetEnvironmentValue(ctx context.Context, environmentID uuid.UUID, key string, showSensitive bool) (*model.EnvironmentValue, error) {
	ev, err := querier(ctx).GetEnvironmentValue(ctx, environmentsql.GetEnvironmentValueParams{
		EnvironmentID: environmentID,
		Key:           key,
		Showsensitive: showSensitive,
	})
	if err != nil {
		return nil, err
	}

	return &model.EnvironmentValue{
		EnvironmentID: ev.EnvironmentID,
		Key:           ev.Key,
		Value:         ev.Value,
		Secret:        ev.Secret,
	}, nil
}

func ListEnvironmentValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) ([]*model.EnvironmentValue, error) {
	values, err := querier(ctx).ListEnvironmentValuesForEnvironment(ctx, environmentsql.ListEnvironmentValuesForEnvironmentParams{
		EnvironmentID: envID,
		Showsensitive: showSensitive,
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.EnvironmentValue, len(values))
	for i, ev := range values {
		ret[i] = &model.EnvironmentValue{
			EnvironmentID: ev.EnvironmentID,
			Key:           ev.Key,
			Value:         ev.Value,
			Secret:        ev.Secret,
		}
	}

	return ret, nil
}

func ListEnvironmentValuesForKey(ctx context.Context, key string) ([]environmentsql.ListEnvironmentValuesForKeyRow, error) {
	return querier(ctx).ListEnvironmentValuesForKey(ctx, key)
}

func DeleteEnvironmentValue(ctx context.Context, environmentID uuid.UUID, key string) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err := querier(ctx).DeleteEnvironmentValue(ctx, environmentsql.DeleteEnvironmentValueParams{
			EnvironmentID: environmentID,
			Key:           key,
		})
		if err != nil {
			return fmt.Errorf("failed to delete environment value: %w", err)
		}

		return audit.Create(ctx, audit.CreateParams{
			Description: "deleted",
			ObjectType:  "environment_values",
			Metadata:    environmentID.String() + ":" + key,
		})
	})
}
