package contextloader

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/cost"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type LoaderFunc func(context.Context) context.Context

func NewLoaderFunc(
	pool *pgxpool.Pool,
	deploymentPublisher deployment.NewPublisher,
	meter metric.Meter,
	log logrus.FieldLogger,
) (LoaderFunc, error) {
	deploymentManager, err := deployment.NewManager(pool, deploymentPublisher, meter, log)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) context.Context {
		ctx = dbtx.Register(ctx, pool)
		ctx = audit.Register(ctx, pool, log)
		ctx = cost.Register(ctx, pool)
		ctx = environment.Register(ctx, pool)
		ctx = feature.Register(ctx, pool)
		ctx = naisdstatus.Register(ctx, pool)
		ctx = deployment.Register(ctx, deploymentManager)
		return ctx
	}, nil
}
