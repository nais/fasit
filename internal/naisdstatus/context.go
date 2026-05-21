package naisdstatus

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatussql"
)

type ctxKey int

// QuerierKey is exposed so tests can inject fake queriers on the context.
const QuerierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, QuerierKey, naisdstatussql.New(pool))
}

func querier(ctx context.Context) naisdstatussql.Querier {
	return ctx.Value(QuerierKey).(naisdstatussql.Querier)
}
