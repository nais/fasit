package deployment

import (
	"context"
	"time"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Manager struct {
	deployer        *deployer
	reconciler      *reconciler
	querier         deploymentsql.Querier
	chartDownloader ChartDownloader
}

type ChartDownloader func(chartURL, version string) (*model.Feature, error)

// TODO: check if we can use same request as in graphql
type Request struct {
	Chart       string             `json:"chart"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Ref         *model.GHRef       `json:"ref"`
	Global      bool               `json:"global"`
	Target      environment.Labels `json:"target"`
	SkipCI      bool               `json:"skipCI"`
}

type Option func(*Manager)

func WithChartDownloader(downloader ChartDownloader) Option {
	return func(m *Manager) {
		m.chartDownloader = downloader
	}
}

func NewManager(repo database.Repo, publisher NewPublisher, m metric.Meter, log logrus.FieldLogger, opts ...Option) (*Manager, error) {
	querier := deploymentsql.New(repo.GetConnPool())
	d, err := newDeployer(repo, querier, publisher, m, log.WithField("subsystem", "deployer"))
	if err != nil {
		return nil, err
	}

	r, err := newReconciler(repo, querier, d, m, log.WithField("subsystem", "reconciler"))
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		deployer:   d,
		reconciler: r,
		querier:    querier,
	}
	for _, opt := range opts {
		opt(mgr)
	}

	if mgr.chartDownloader == nil {
		mgr.chartDownloader = model.FromChart
	}

	return mgr, nil
}

func (dm *Manager) Run(ctx context.Context, interval time.Duration) {
	dm.reconciler.Run(ctx, interval)
}

// Reconcile performs a reconciliation of deployments, and will block until complete.
func (dm *Manager) Reconcile(ctx context.Context) error {
	return dm.reconciler.Reconcile(ctx)
}
