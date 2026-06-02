package featureassignment

import (
	"context"

	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
)

type ctxKey int

const managerKey ctxKey = iota

func Register(ctx context.Context, manager *Manager) context.Context {
	return context.WithValue(ctx, managerKey, manager)
}

func RegisterForTest(ctx context.Context, querier featureassignmentsql.Querier) context.Context {
	return context.WithValue(ctx, managerKey, &Manager{querier: querier})
}

func fromContext(ctx context.Context) *Manager {
	return ctx.Value(managerKey).(*Manager)
}

func querier(ctx context.Context) featureassignmentsql.Querier {
	q := fromContext(ctx).querier
	if tx, ok := dbtx.Tx(ctx); ok {
		if real, ok := q.(*featureassignmentsql.Queries); ok {
			return real.WithTx(tx)
		}
	}
	return q
}

// TODO: remove this when looking at the workers package
func GetManager(ctx context.Context) *Manager {
	return fromContext(ctx)
}
