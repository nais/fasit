package fasit

import (
	"context"
	"time"

	"github.com/nais/fasit/internal/cluster"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/slack"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func runClusterUpgrader(ctx context.Context, slackChannel string, log logrus.FieldLogger, clusterManager cluster.ClusterManager, repo database.Repo, meter metric.Meter, slack slack.SlackClient) error {
	s := workers.NewScheduler(log)
	clusterUpgrader := cluster.NewClusterUpgrader(repo, log, clusterManager, meter, slack, slackChannel)
	autoUpgrader := cluster.NewAutoUpgrader(repo, log, clusterManager, meter)

	s.Register("cluster-upgrader", clusterUpgrader, 30*time.Second)
	s.Register("auto-upgrader", autoUpgrader, 1*time.Hour)
	s.Start(ctx)

	return nil
}

func clustersMetrics(ctx context.Context, repo database.Repo, meter metric.Meter, log logrus.FieldLogger) {
	log = log.WithField("subsystem", "cluster-info")

	gauge, err := meter.Int64Gauge("cluster_info")
	if err != nil {
		return
	}

	for {
		tenants, err := repo.TenantsGet(ctx)
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
