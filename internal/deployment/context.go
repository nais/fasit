package deployment

import (
	"context"

	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
)

type ctxKey int

const managerKey ctxKey = iota

func Register(ctx context.Context, deploymentManager *Manager) context.Context {
	return context.WithValue(ctx, managerKey, deploymentManager)
}

func RegisterForTest(ctx context.Context, querier deploymentsql.Querier) context.Context {
	return context.WithValue(ctx, managerKey, &Manager{querier: querier})
}

func fromContext(ctx context.Context) *Manager {
	return ctx.Value(managerKey).(*Manager)
}

func querier(ctx context.Context) deploymentsql.Querier {
	q := fromContext(ctx).querier
	if tx, ok := dbtx.Tx(ctx); ok {
		if real, ok := q.(*deploymentsql.Queries); ok {
			return real.WithTx(tx)
		}
	}
	return q
}

// TODO: remove this when looking at the workers package
func GetManager(ctx context.Context) *Manager {
	return fromContext(ctx)
}
