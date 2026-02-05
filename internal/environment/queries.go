package environment

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type Manager struct {
	querier environmentsql.Querier
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{
		querier: environmentsql.New(pool),
	}
}

func (m *Manager) TenantEnvironments(ctx context.Context, onlyReconciled bool) ([]*model.TenantEnvironment, error) {
	data, err := m.querier.TenantEnvironments(ctx, !onlyReconciled)
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

func (m *Manager) ListCIEnvironmentsForTarget(ctx context.Context, labels Labels) ([]*model.TenantEnvironment, error) {
	envs, err := m.querier.ListCIEnvironmentsForTarget(ctx, types.EnvironmentLabels(labels))
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

func (m *Manager) ListLabels(ctx context.Context, environmentID uuid.UUID) ([]*model.EnvironmentLabel, error) {
	labels, err := m.querier.GetLabels(ctx, environmentID)
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

func (m *Manager) Get(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	env, err := m.querier.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func (m *Manager) GetTenant(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	tenant, err := m.querier.GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}
	return tenantFromSQL(tenant), nil
}
