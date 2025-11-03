package deployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type ReconcilerStore interface {
	DeploymentTargetsGet(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
}

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

	for _, d := range deployments {
		if err := r.reconcileDeployment(ctx, d); err != nil {
			r.log.WithError(err).WithField("deployment", d.ID).Error("reconcile deployment")
		}
	}
	return nil
}

func (r *Reconciler) reconcileDeployment(ctx context.Context, _ gensql.Deployment) error {
	deploymentTargets, err := r.repo.DeploymentTargetsGet(ctx)
	if err != nil {
		return fmt.Errorf("get deployment targets: %w", err)
	}

	_ = deploymentTargets

	return nil
}
