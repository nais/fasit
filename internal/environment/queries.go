package environment

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

const managerKey ctxKey = iota

type Manager struct {
	querier environmentsql.Querier
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{
		querier: environmentsql.New(pool),
	}
}

func NewContext(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, managerKey, NewManager(pool))
}

func fromContext(ctx context.Context) *Manager {
	return ctx.Value(managerKey).(*Manager)
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

func TenantEnvironments(ctx context.Context, onlyReconciled bool) ([]*model.TenantEnvironment, error) {
	return fromContext(ctx).TenantEnvironments(ctx, onlyReconciled)
}
