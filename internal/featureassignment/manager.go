package featureassignment

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type Manager struct {
	querier featureassignmentsql.Querier
	log     *slog.Logger
	pool    *pgxpool.Pool
}

type ChartDownloaderFunc func(chartURL, version string) (*model.Feature, error)

// Override for testing
var ChartDownloader = func(chartURL, version string) (*model.Feature, error) {
	return model.FromChart(chartURL, version)
}

type Option func(*Manager)

func NewManager(pool *pgxpool.Pool, log *slog.Logger, opts ...Option) (*Manager, error) {
	querier := featureassignmentsql.New(pool)

	mgr := &Manager{
		pool:    pool,
		querier: querier,
		log:     log.With("subsystem", "featureassignment-manager"),
	}
	for _, opt := range opts {
		opt(mgr)
	}

	return mgr, nil
}
