package reconciler

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

type ctxKey struct{}

// WithContext returns a new context with the reconciler stored in it.
func WithContext(ctx context.Context, r *Reconciler) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext returns the reconciler stored in the context, or nil.
func FromContext(ctx context.Context) *Reconciler {
	r, _ := ctx.Value(ctxKey{}).(*Reconciler)
	return r
}

type querierKey struct{}

// Register stores a reconciler querier in the context for the read path
// (ReconcileStatuses). Wired by contextloader for every request.
func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, querierKey{}, reconcilersql.New(pool))
}

// querier returns the request-scoped reconciler querier, bound to the current
// transaction when one is active (dbtx), so the read path participates in
// caller transactions.
func querier(ctx context.Context) reconcilersql.Querier {
	q := ctx.Value(querierKey{}).(reconcilersql.Querier)
	if tx, ok := dbtx.Tx(ctx); ok {
		if real, ok := q.(*reconcilersql.Queries); ok {
			return real.WithTx(tx)
		}
	}
	return q
}
