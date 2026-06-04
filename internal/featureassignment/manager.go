package featureassignment

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
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

func (m *Manager) Receive(ctx context.Context, status *message.Helm) error {
	di, err := GetDeployInstruction(ctx, status.DIID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			m.log.With("diid", status.DIID).Warn("unknown deploy instruction")
			return nil
		}
		return err
	}

	if di.FeatureAssignmentID != nil {
		msg := "received status from naisd."
		if status.Error != "" {
			msg += " error: " + status.Error
		}
		err := m.querier.SetReconcileStatus(ctx, featureassignmentsql.SetReconcileStatusParams{
			FeatureAssignmentID: *di.FeatureAssignmentID,
			EnvironmentID:       di.EnvironmentID,
			Status:              status.RolloutStatus.String(),
			Message:             msg,
		})
		if err != nil {
			m.log.With("err", err,
				"feature_assignment_id", di.FeatureAssignmentID,
				"environment_id", di.EnvironmentID,
				"status", status.RolloutStatus,
				"msg", msg).Error("create feature assignment status")
		}
	}
	return nil
}
