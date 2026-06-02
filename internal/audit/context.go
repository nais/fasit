package audit

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/dbtx"
)

type ctxKey int

const (
	// QuerierKey is exposed so tests can inject fake queriers on the context.
	QuerierKey ctxKey = iota
	logKey
)

func Register(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) context.Context {
	ctx = context.WithValue(ctx, QuerierKey, auditsql.Querier(auditsql.New(pool)))
	ctx = context.WithValue(ctx, logKey, log)
	return ctx
}

func RegisterTestDeps(ctx context.Context, q auditsql.Querier, log *slog.Logger) context.Context {
	ctx = context.WithValue(ctx, QuerierKey, q)
	ctx = context.WithValue(ctx, logKey, log)
	return ctx
}

func querier(ctx context.Context) auditsql.Querier {
	q := ctx.Value(QuerierKey).(auditsql.Querier)
	if tx, ok := dbtx.Tx(ctx); ok {
		if r, ok := q.(*auditsql.Queries); ok {
			return r.WithTx(tx)
		}
	}
	return q
}

func logger(ctx context.Context) *slog.Logger {
	return ctx.Value(logKey).(*slog.Logger)
}
