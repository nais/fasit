package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	database.HealthRepo
	database.DeployInstructionRepo
}

type (
	Publisher    interface{}
	NewPublisher func(topicID string, log *logrus.Entry) Publisher
)

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

func (r *Reconciler) shouldDeployToEnvironment(ctx context.Context, deployment gensql.DeploymentsForEnvironmentRow, environment *model.TenantEnvironment) (bool, error) {
	existingDeploy, err := r.repo.DeployInstructionsLatestForFeature(ctx, environment.ID, deployment.Deployment.FeatureName)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("get deploy instructions latest for environment %q: %w", environment.Name, err)
		}
	}

	if existingDeploy != nil {
		// TODO: should we check version as well?
		if existingDeploy.Hash == deployment.Deployment.Hash {
			r.log.Debug("deployment is already up to date - skip reconcile")
			return false, nil
		}
	}

	return r.isDependenciesDeployed(ctx, deployment, environment.ID)
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, environment *model.TenantEnvironment) error {
	health, err := r.repo.HealthGet(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}
	if time.Since(health.ReportedAt) > 3*time.Minute {
		r.log.Debug("naisd is unhealthy - skip reconcile")
		return nil
	}

	allDeployments, err := r.repo.DeploymentsForEnvironment(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	for _, deployment := range filterDeployments(allDeployments) {

		if ok, err := r.shouldDeployToEnvironment(ctx, deployment, environment); err != nil {
			return err
		} else if !ok {
			continue
		}

		// TODO: should we use deploy_instructions to keep track of deployed features or deployment_target?
		if err = r.createDeploymentTarget(ctx, deployment.Deployment, environment.ID); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) isDependenciesDeployed(ctx context.Context, deployment gensql.DeploymentsForEnvironmentRow, envID uuid.UUID) (bool, error) {
	deps, err := getDependencies(deployment.FeatureDatum.Dependencies)
	if err != nil {
		return false, err
	}

	if len(deps) == 0 {
		return true, nil
	}

	for _, dep := range deps {
		if len(dep.AllOf) > 0 {
			missing, err := r.repo.DeployInstructionsGetFeaturesNotInEnv(ctx, dep.AllOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}
			if len(missing) > 0 {
				r.log.
					WithField("environment_id", envID.String()).
					WithField("features", strings.Join(missing, ",")).
					Debug("feature dependencies not found in env")
				return false, nil
			}
		}
		if len(dep.AnyOf) > 0 {
			missing, err := r.repo.DeployInstructionsGetFeaturesNotInEnv(ctx, dep.AnyOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}
			if len(missing) == len(dep.AnyOf) {
				r.log.
					WithField("environment_id", envID.String()).
					WithField("features", strings.Join(missing, ",")).
					Debug("feature dependencies not found in env")
				return false, nil
			}
		}
	}

	return true, nil
}

// filterDeployments filters the deployments to only include the most specific deployment with the latest created
// timestamp.
func filterDeployments(allDeployments []gensql.DeploymentsForEnvironmentRow) []gensql.DeploymentsForEnvironmentRow {
	deployments := map[string]gensql.DeploymentsForEnvironmentRow{}
	for _, row := range allDeployments {
		d, ok := deployments[row.Deployment.FeatureName]
		if !ok {
			deployments[row.Deployment.FeatureName] = row
			continue
		}

		if len(row.Deployment.Target) > len(d.Deployment.Target) {
			deployments[row.Deployment.FeatureName] = row
			continue
		}

		if len(row.Deployment.Target) == len(d.Deployment.Target) && row.Deployment.Created.Time.After(d.Deployment.Created.Time) {
			deployments[row.Deployment.FeatureName] = row
			continue
		}
	}

	ret := make([]gensql.DeploymentsForEnvironmentRow, 0)
	for _, d := range deployments {
		ret = append(ret, d)
	}
	return ret
}

func getDependencies(deps []byte) (model.Dependencies, error) {
	ret := model.Dependencies{}
	if err := json.Unmarshal(deps, &ret); err != nil {
		return nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}
	return ret, nil
}
