package featureassignment

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	"go.opentelemetry.io/otel/metric"
)

type Manager struct {
	deployer *deployer
	querier  featureassignmentsql.Querier
	log      *slog.Logger
	pool     *pgxpool.Pool
}

type ChartDownloaderFunc func(chartURL, version string) (*model.Feature, error)

// Override for testing
var ChartDownloader = func(chartURL, version string) (*model.Feature, error) {
	return model.FromChart(chartURL, version)
}

type Option func(*Manager)

func NewManager(pool *pgxpool.Pool, publisher NewPublisher, m metric.Meter, log *slog.Logger, opts ...Option) (*Manager, error) {
	querier := featureassignmentsql.New(pool)
	d, err := newDeployer(pool, querier, publisher, m, log.With("subsystem", "featureassignment-deployer"))
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		deployer: d,
		pool:     pool,
		querier:  querier,
		log:      log.With("subsystem", "featureassignment-manager"),
	}
	for _, opt := range opts {
		opt(mgr)
	}

	return mgr, nil
}
