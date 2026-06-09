package featureassignment

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
)

type ctxKey int

const querierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, querierKey, featureassignmentsql.New(pool))
}

func querier(ctx context.Context) featureassignmentsql.Querier {
	q := ctx.Value(querierKey).(featureassignmentsql.Querier)
	if tx, ok := dbtx.Tx(ctx); ok {
		if r, ok := q.(*featureassignmentsql.Queries); ok {
			return r.WithTx(tx)
		}
	}
	return q
}
