package fasit

import (
	"context"
	"time"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func clustersMetrics(ctx context.Context, repo database.Repo, meter metric.Meter, log logrus.FieldLogger) {
	log = log.WithField("subsystem", "cluster-info")

	gauge, err := meter.Int64Gauge("cluster_info")
	if err != nil {
		return
	}

	for {
		tenants, err := environment.GetTenants(ctx)
		if err != nil {
			log.WithError(err).Error("getting tenants")
			return
		}

		for _, tenant := range tenants {
			environments, err := repo.EnvironmentsGet(ctx, tenant.ID)
			if err != nil {
				log.WithError(err).Error("getting environments")
				return
			}

			for _, env := range environments {
				attr := []attribute.KeyValue{
					attribute.String("tenant", tenant.Name),
					attribute.String("environment", env.Name),
					attribute.String("kind", env.Kind.String()),
				}
				gauge.Record(ctx, 1, metric.WithAttributes(attr...))
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Minute):
		}
	}
}
