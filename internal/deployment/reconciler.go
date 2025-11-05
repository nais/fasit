package deployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type ReconcilerStore interface {
	database.EnvironmentRepo
	database.DeploymentRepo
}

type labelMap = map[string]string

type Publisher interface{}

type NewPublisher func(topicID string, log *logrus.Entry) Publisher

type Notifier interface{}

type Reconciler struct {
	repo      ReconcilerStore
	publisher NewPublisher
	log       *logrus.Entry
	notifier  Notifier

	lock    sync.Mutex
	running bool

	// Metrics
	reconcileTime  metric.Int64Histogram
	deployMessages metric.Int64Counter
}

func NewReconciler(
	repo ReconcilerStore,
	publisher NewPublisher,
	notifier Notifier,
	meter metric.Meter,
	log *logrus.Entry,
) (*Reconciler, error) {
	return &Reconciler{
		repo:      repo,
		publisher: publisher,
		log:       log,
		notifier:  notifier,
	}, nil
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		r.log.Debug("reconciling")
		if err := r.Reconcile(ctx); err != nil {
			r.log.WithError(err).Error("reconcile")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Reconciler) Reconcile(ctx context.Context) error {
	ctx = auth.SetEmail(ctx, "system:deployment_reconciler")

	if shouldrun := r.lock.TryLock(); !shouldrun {
		return nil
	}
	defer r.lock.Unlock()

	deployments, err := r.repo.DeploymentsGet(ctx)
	if err != nil {
		return fmt.Errorf("get deployments: %w", err)
	}

	for _, deployment := range deployments {
		envs, err := r.repo.EnvironmentsTargetedByDeployment(ctx, deployment.ID)
		if err != nil {
			return fmt.Errorf("get environments targeted by deployment: %w", err)
		}

		for _, envID := range envs {
			if err := r.createDeploymentTarget(ctx, deployment, envID); err != nil {
				r.log.
					WithError(err).
					WithFields(logrus.Fields{
						"deployment_id":  deployment.ID,
						"environment_id": envID,
					}).
					Error("reconcile deployment")
			}
		}
	}

	return nil
}

func (r *Reconciler) createDeploymentTarget(ctx context.Context, d gensql.Deployment, envId uuid.UUID) error {
	err := r.repo.DeploymentTargetsCreate(ctx, d.ID, envId)
	if err != nil {
		return fmt.Errorf("create deployment target: %w", err)
	}

	return nil
}
