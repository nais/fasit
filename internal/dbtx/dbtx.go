// Package dbtx provides a small, generic mechanism for running a function
// inside a database transaction whose handle is propagated via context.
//
// Domain packages that want to participate look up an active transaction
// via Tx(ctx); callers that want to scope multiple domain calls into a
// single transaction wrap them in WithTx.
package dbtx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey int

const (
	poolKey ctxKey = iota
	txKey
)

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, poolKey, pool)
}

func Tx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey).(pgx.Tx)
	return tx, ok
}

// WithTx runs fn inside a transaction. If no pool is registered on ctx, fn
// is invoked directly with the original context so unit tests that inject
// mock queriers keep working. Nested WithTx calls reuse the outer tx.
func WithTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := Tx(ctx); ok {
		return fn(ctx)
	}

	pool, ok := ctx.Value(poolKey).(*pgxpool.Pool)
	if !ok || pool == nil {
		return fn(ctx)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(context.WithValue(ctx, txKey, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
