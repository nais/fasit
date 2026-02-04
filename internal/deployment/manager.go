package deployment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Manager struct {
	deployer        *deployer
	reconciler      *reconciler
	querier         deploymentsql.Querier
	chartDownloader ChartDownloader
	log             logrus.FieldLogger
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
	d, err := newDeployer(repo, querier, publisher, m, log.WithField("subsystem", "deployment-deployer"))
	if err != nil {
		return nil, err
	}

	r, err := newReconciler(repo, querier, d, m, log.WithField("subsystem", "deployment-reconciler"))
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		deployer:   d,
		reconciler: r,
		querier:    querier,
		log:        log.WithField("subsystem", "deployment-manager"),
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

func (dm *Manager) Receive(ctx context.Context, status *message.Helm) error {
	di, err := dm.querier.DeployInstructionsByID(ctx, status.DIID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dm.log.WithField("diid", status.DIID).Warn("unknown deploy instruction")
			return nil
		}
		return err
	}

	if di.DeploymentID != nil {
		msg := "received status from naisd."
		if status.Error != "" {
			msg += " error: " + status.Error
		}
		err := dm.querier.SetDeploymentStatus(ctx, deploymentsql.SetDeploymentStatusParams{
			DeploymentID:  *di.DeploymentID,
			EnvironmentID: di.EnvironmentID,
			Status:        status.RolloutStatus.String(),
			Message:       msg,
		})
		if err != nil {
			dm.log.WithFields(logrus.Fields{
				"deployment_id":  di.DeploymentID,
				"environment_id": di.EnvironmentID,
				"status":         status.RolloutStatus,
				"msg":            msg,
			}).WithError(err).Error("create deployment status")
		}
	}
	return nil
}
