package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

func Create(ctx context.Context, env *EnvironmentCreate) (*Environment, error) {
	var ret environmentsql.Environment
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error

		tenant, err := GetTenant(ctx, env.TenantID)
		if err != nil {
			return fmt.Errorf("get tenant for labels: %w", err)
		}

		if len(env.Labels) == 0 {
			env.Labels = make(map[string]string)
		}
		env.Labels["name"] = env.Name
		env.Labels["tenant"] = tenant.Name

		ret, err = querier(ctx).CreateEnvironment(ctx, environmentsql.CreateEnvironmentParams{
			Name:        env.Name,
			Description: env.Description,
			TenantID:    env.TenantID,
			Kind:        types.EnvironmentKind(env.Kind),
			Labels:      env.Labels,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionCreated,
			ObjectType: audit.ObjectTypeEnvironment,
			ObjectID:   tenant.Name + "/" + env.Name,
		})
	})
	if err != nil {
		return nil, err
	}

	return environmentFromSQL(ret), nil
}

func SetLabels(ctx context.Context, environmentID uuid.UUID, labels Labels) error {
	existing, err := querier(ctx).GetEnvironmentLabels(ctx, environmentID)
	if err != nil {
		existing = nil
	}

	lbls := make(types.EnvironmentLabels)
	maps.Copy(lbls, existing)
	maps.Copy(lbls, labels)

	err = querier(ctx).SetEnvironmentLabels(ctx, environmentsql.SetEnvironmentLabelsParams{
		Labels: lbls,
		ID:     environmentID,
	})
	if err != nil {
		return err
	}

	return audit.Create(ctx, audit.CreateParams{
		Action:        audit.ActionUpdated,
		ObjectType:    audit.ObjectTypeEnvironment,
		ObjectID:      "labels",
		EnvironmentID: &environmentID,
		Metadata:      labels,
	})
}

func GetLabels(ctx context.Context, environmentID uuid.UUID) (Labels, error) {
	labels, err := querier(ctx).GetEnvironmentLabels(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	return Labels(labels), nil
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
			Action:        audit.ActionUpdated,
			ObjectType:    audit.ObjectTypeEnvironmentValue,
			ObjectID:      key,
			EnvironmentID: &environmentID,
			Metadata: map[string]any{
				"secret": secret,
			},
		})
	})
}

func ListTenantEnvironments(ctx context.Context, onlyReconciled bool) ([]*TenantEnvironment, error) {
	data, err := querier(ctx).ListTenantEnvironments(ctx, !onlyReconciled)
	if err != nil {
		return nil, err
	}

	var ret []*TenantEnvironment
	for _, d := range data {
		ret = append(ret, &TenantEnvironment{
			Environment: Environment{
				ID:           d.ID,
				Name:         d.Name,
				Description:  d.Description,
				Created:      d.Created,
				LastModified: d.LastModified,
				Kind:         EnvironmentKind(d.Kind),
			},
			TenantName: d.TenantName,
			TenantID:   d.TenantID,
		})
	}

	return ret, nil
}

func Get(ctx context.Context, id uuid.UUID) (*Environment, error) {
	env, err := querier(ctx).GetEnvironment(ctx, id)
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*Environment, error) {
	env, err := querier(ctx).GetEnvironmentByName(ctx, environmentsql.GetEnvironmentByNameParams{
		TenantID: tenantID,
		Name:     name,
	})
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func GetTenant(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	tenant, err := querier(ctx).GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func GetTenantByName(ctx context.Context, name string) (*Tenant, error) {
	tenant, err := querier(ctx).GetTenantByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func CreateTenant(ctx context.Context, t *TenantCreate) (*Tenant, error) {
	var tenant environmentsql.Tenant
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		tenant, err = querier(ctx).CreateTenant(ctx, environmentsql.CreateTenantParams{
			Name:        t.Name,
			Description: t.Description,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionCreated,
			ObjectType: audit.ObjectTypeTenant,
			ObjectID:   t.Name,
		})
	})
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}

func List(ctx context.Context, tenantID uuid.UUID) ([]*Environment, error) {
	envs, err := querier(ctx).ListEnvironments(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	environmentSlice := make([]*Environment, len(envs))
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
			Action:        audit.ActionDeleted,
			ObjectType:    audit.ObjectTypeEnvironmentValue,
			ObjectID:      key,
			EnvironmentID: &environmentID,
		})
	})
}
