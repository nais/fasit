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
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type ReconcilerStore interface {
	database.EnvironmentRepo
	database.DeploymentRepo
	database.TenantRepo
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

	tenants, err := r.repo.TenantsGet(ctx)
	if err != nil {
		return fmt.Errorf("get tenants: %w", err)
	}

	for _, tenant := range tenants {
		environments, err := r.repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			return fmt.Errorf("get environments for tenant %q: %w", tenant.Name, err)
		}

		for _, environment := range environments {
			if err := r.reconcileEnvironment(ctx, environment); err != nil {
				return fmt.Errorf("reconcile environment %q for tenant: %q: %w", environment.Name, tenant.Name, err)
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

func (r *Reconciler) shouldDeployToEnvironment(ctx context.Context, deployment gensql.Deployment, envID uuid.UUID) bool {
	enabled, err := r.repo.FeatureEnabled(ctx, deployment.FeatureName, envID)
	if err != nil {
		r.log.WithError(err).Errorf("get feature state for deployment %q", deployment.ID)
		return false
	}

	if !enabled {
		return false
	}

	// Additional criteria can be added here
	return true
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, environment *model.Environment) error {
	allDeployments, err := r.repo.DeploymentsForEnvironment(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	for _, deployment := range filterDeployments(allDeployments) {
		if !r.shouldDeployToEnvironment(ctx, deployment, environment.ID) {
			continue
		}

		if err := r.createDeploymentTarget(ctx, deployment, environment.ID); err != nil {
			r.log.
				WithError(err).
				WithFields(logrus.Fields{
					"deployment_id":  deployment.ID,
					"environment_id": environment.ID,
				}).
				Error("reconcile deployment")
		}
	}

	return nil
}

// filterDeployments filters the deployments to only include the most specific deployment with the latest created
// timestamp.
func filterDeployments(allDeployments []gensql.Deployment) []gensql.Deployment {
	deployments := map[string]gensql.Deployment{}
	for _, deployment := range allDeployments {
		d, ok := deployments[deployment.FeatureName]
		if !ok {
			deployments[deployment.FeatureName] = deployment
			continue
		}

		if len(deployment.Target) > len(d.Target) {
			deployments[deployment.FeatureName] = deployment
			continue
		}

		if len(deployment.Target) == len(d.Target) && deployment.Created.Time.After(d.Created.Time) {
			deployments[deployment.FeatureName] = deployment
			continue
		}
	}

	ret := make([]gensql.Deployment, 0)
	for _, d := range deployments {
		ret = append(ret, d)
	}
	return ret
}
