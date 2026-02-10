package environment

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

const querierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, querierKey, environmentsql.New(pool))
}

func querier(ctx context.Context) environmentsql.Querier {
	return ctx.Value(querierKey).(environmentsql.Querier)
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
				CI:           d.Ci,
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

func ListCIEnvironmentsForTarget(ctx context.Context, labels Labels) ([]*model.TenantEnvironment, error) {
	envs, err := querier(ctx).ListCIEnvironmentsForTarget(ctx, types.EnvironmentLabels(labels))
	if err != nil {
		return nil, err
	}

	ret := make([]*model.TenantEnvironment, len(envs))
	for i, e := range envs {
		ret[i] = &model.TenantEnvironment{
			Environment: model.Environment{
				ID:           e.Environment.ID,
				Name:         e.Environment.Name,
				CI:           e.Environment.Ci,
				Description:  e.Environment.Description,
				Created:      e.Environment.Created.Time,
				LastModified: e.Environment.LastModified.Time,
				Kind:         model.EnvironmentKind(e.Environment.Kind),
			},
			TenantName: e.TenantName,
			TenantID:   e.Environment.TenantID,
		}
	}
	return ret, nil
}

func ListLabels(ctx context.Context, environmentID uuid.UUID) ([]*model.EnvironmentLabel, error) {
	labels, err := querier(ctx).GetLabels(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	var ret []*model.EnvironmentLabel
	for k, v := range labels {
		ret = append(ret, &model.EnvironmentLabel{
			Key:   k,
			Value: v,
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
