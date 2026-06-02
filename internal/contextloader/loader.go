package contextloader

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type LoaderFunc func(context.Context) context.Context

func NewLoaderFunc(
	pool *pgxpool.Pool,
	publisher featureassignment.NewPublisher,
	meter metric.Meter,
	log logrus.FieldLogger,
) (LoaderFunc, error) {
	manager, err := featureassignment.NewManager(pool, publisher, meter, log)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) context.Context {
		ctx = dbtx.Register(ctx, pool)
		ctx = audit.Register(ctx, pool, log)
		ctx = environment.Register(ctx, pool)
		ctx = feature.Register(ctx, pool)
		ctx = naisdstatus.Register(ctx, pool)
		ctx = uidata.Register(ctx, pool)
		ctx = featureassignment.Register(ctx, manager)
		return ctx
	}, nil
}
