package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/ui/database/sqlgen"
)

type ctxKey int

const querierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	ctx = context.WithValue(ctx, querierKey, sqlgen.New(pool))
	return ctx
}

func querier(ctx context.Context) sqlgen.Querier {
	q := ctx.Value(querierKey).(sqlgen.Querier)
	if tx, ok := dbtx.Tx(ctx); ok {
		if r, ok := q.(*sqlgen.Queries); ok {
			return r.WithTx(tx)
		}
	}
	return q
}
