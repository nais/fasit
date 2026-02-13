package environment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

// QuerierKey is exposed for testing to override querier with mocks.
// Avoid usage by e.g. using testcontainers.
const QuerierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, QuerierKey, environmentsql.New(pool))
}

func querier(ctx context.Context) environmentsql.Querier {
	return ctx.Value(QuerierKey).(environmentsql.Querier)
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

func Warnings(ctx context.Context, environmentID *uuid.UUID, tenantID *uuid.UUID) ([]model.Warning, error) {
	args := environmentsql.WarningsParams{}
	if environmentID == nil && tenantID == nil {
		return nil, fmt.Errorf("must specify either environmentID or tenantID")
	}
	if environmentID != nil && tenantID != nil {
		return nil, fmt.Errorf("must specify either environmentID or tenantID, not both")
	}

	if environmentID != nil {
		args.EnvironmentID = *environmentID
	}
	if tenantID != nil {
		args.TenantID = *tenantID
	}

	warnings, err := querier(ctx).Warnings(ctx, args)
	if err != nil {
		return nil, err
	}

	// Ensure that warnings are only returned for features that are actually in the environment
	ws := []environmentsql.WarningsRow{}
	for _, w := range warnings {
		if w.FeatureDataName != "" {
			ws = append(ws, w)
		}
	}

	return warningsFromSQL(ws)
}

func warningsFromSQL(warnings []environmentsql.WarningsRow) ([]model.Warning, error) {
	var result []model.Warning
	for _, w := range warnings {
		switch w.Type {
		case "feature_status":
			result = append(result, model.FeatureWarning{
				Message:       "feature not reconciled correctly",
				EnvironmentID: w.EnvironmentID,
				FeatureName:   w.FeatureName,
			})

		case "naisd":
			result = append(result, model.NaisdWarning{
				Message:       "naisd not healthy",
				EnvironmentID: w.EnvironmentID,
			})
		default:
			return nil, fmt.Errorf("unknown warning type: %s", w.Type)
		}
	}
	return result, nil
}
