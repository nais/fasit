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
	database.DeploymentRepo
	database.TenantRepo
	database.FeaturesRepo
	database.FeatureStateRepo
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

	tenantEnvironments, err := r.repo.TenantEnvironments(ctx, true)
	if err != nil {
		return fmt.Errorf("get tenant environments: %w", err)
	}

	for _, environment := range tenantEnvironments {
		if err := r.reconcileEnvironment(ctx, environment); err != nil {
			return fmt.Errorf("reconcile environment %q for tenant: %q: %w", environment.Name, environment.TenantName, err)
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

func (r *Reconciler) shouldDeployToEnvironment(ctx context.Context, feature *model.Feature, envID uuid.UUID) bool {
	if len(feature.Dependencies) > 0 {
		states, err := r.repo.FeatureStatesGet(ctx, envID)
		if err != nil {
			// TODO: log
			return false
		}

		// TODO: no need to fetch all?
		enabledFeatures := []string{}
		for _, state := range states {
			if state.Enabled {
				enabledFeatures = append(enabledFeatures, state.FeatureName)
			}
		}

		missingFeatures := feature.Dependencies.FindMissing(enabledFeatures)
		if len(missingFeatures) > 0 {
			return false // TODO: log => nil, fmt.Errorf("dependency '%v' not enabled", missingFeatures)
		}
	}

	return true
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, environment *model.TenantEnvironment) error {
	allDeployments, err := r.repo.DeploymentsForEnvironment(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	for _, deployment := range filterDeployments(allDeployments) {
		feature, err := r.repo.FeatureByNameForEnv(ctx, deployment.FeatureName, environment.ID)
		if err != nil {
			return err
		}

		if !r.shouldDeployToEnvironment(ctx, feature, environment.ID) {
			continue
		}

		if _, err = r.repo.FeatureStatesCreateOrUpdate(ctx, environment.ID, feature, true); err != nil {
			return fmt.Errorf("enable feature %q for environment %q: %w", deployment.FeatureName, environment.Name, err)
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
