package deployment

import (
	"context"
	"encoding/json"
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
	DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target database.EnvironmentLabels) error
	DeploymentTargetsGet(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
}

type lookupMap = map[database.EnvironmentLabelKey]map[database.EnvironmentLabelValue]database.EnvironmentID

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

	envs, err := r.repo.EnvironmentsGetIDWithLabels(ctx)
	if err != nil {
		return fmt.Errorf("get environments: %w", err)
	}

	lookup := lookupMap{}
	for _, env := range envs {
		if _, exists := lookup[env.Key]; !exists {
			lookup[env.Key] = map[database.EnvironmentLabelValue]database.EnvironmentID{}
		}
		lookup[env.Key][env.Value] = env.ID
	}

	for _, d := range deployments {
		var target map[string]string
		err = json.Unmarshal(d.Target, &target)
		if err != nil {
			r.log.WithError(err).Error("unmarshal target")
		}

		for k, v := range target {
			id, ok := lookup[k][v]
			if !ok {
				continue
			}
			err = r.createDeploymentTarget(ctx, d, id)
			if err != nil {
				r.log.WithError(err).WithField("deployment_id", d.ID).WithField("environment_id", id).Error("reconcile deployment")
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
