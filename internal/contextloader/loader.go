package contextloader

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/uidata"
)

type LoaderFunc func(context.Context) context.Context

func NewLoaderFunc(pool *pgxpool.Pool, log *slog.Logger) (LoaderFunc, error) {
	manager, err := featureassignment.NewManager(pool, log)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) context.Context {
		ctx = dbtx.Register(ctx, pool)
		ctx = audit.Register(ctx, pool, log)
		ctx = environment.Register(ctx, pool)
		ctx = feature.Register(ctx, pool)
		ctx = naisdstatus.Register(ctx, pool)
		ctx = reconciler.Register(ctx, pool)
		ctx = uidata.Register(ctx, pool)
		ctx = featureassignment.Register(ctx, manager)
		return ctx
	}, nil
}
