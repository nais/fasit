package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
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
	database.ConfigRepo
}

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log logrus.FieldLogger) Publisher

type ReconcileTriggerEvent struct {
	DeploymentID   uuid.UUID
	FeatureName    string
	FeatureVersion string
	Type           ReconcileTriggerEventType
}

type ReconcileTriggerEventType int

const (
	ReconcileTriggerEventTypeNewDeployment = iota
	ReconcileTriggerEventTypeGlobalConfigUpdate
	ReconcileTriggerEventTypeFeatureConfigUpdate
)

type Reconciler struct {
	repo             ReconcilerStore
	publisher        NewPublisher
	log              logrus.FieldLogger
	reconcileTrigger <-chan ReconcileTriggerEvent

	lock    sync.Mutex
	running bool

	// Metrics
	reconcileTime  metric.Int64Histogram
	deployMessages metric.Int64Counter
}

func NewReconciler(
	repo ReconcilerStore,
	publisher NewPublisher,
	reconcileTrigger <-chan ReconcileTriggerEvent,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*Reconciler, error) {
	return &Reconciler{
		repo:             repo,
		publisher:        publisher,
		log:              log,
		reconcileTrigger: reconcileTrigger,
	}, nil
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	for {
		r.log.Debug("reconciling")
		if err := r.Reconcile(ctx); err != nil {
			r.log.WithError(err).Error("reconcile")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		case e := <-r.reconcileTrigger:
			r.log.WithFields(logrus.Fields{
				"deployment_id":   e.DeploymentID,
				"feature_name":    e.FeatureName,
				"feature_version": e.FeatureVersion,
				"type":            e.Type,
			}).Info("manual reconcile triggered")
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

func (r *Reconciler) reconcileEnvironment(ctx context.Context, environment *model.TenantEnvironment) error {
	health, err := r.repo.HealthGet(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}

	// TODO: move health check further down the line so that we can report unhealthy status for each deployment?
	if time.Since(health.ReportedAt) > 3*time.Minute {
		r.log.WithFields(logrus.Fields{
			"tenant":     environment.TenantName,
			"enviroment": environment.Name,
		}).Debug("naisd is unhealthy - skip reconcile")
		return nil
	}

	mgr := r.publisher(naisdTopicID(environment.TenantName, environment.Name), r.log)
	defer mgr.Stop()

	allDeployments, err := r.repo.DeploymentsForEnvironment(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	for _, deployment := range filterDeployments(allDeployments) {
		values, err := r.repo.HelmValues(ctx, deployment.Feature, environment.ID)
		if err != nil {
			var fer *database.ErrMissingRequiredFields
			if errors.As(err, &fer) {
				msg := fmt.Sprintf("missing required chart config: %s", strings.Join(fer.Fields, ", "))
				r.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusFailed, msg)
				continue
			}
			return fmt.Errorf("get helm values for feature: %w", err)
		}

		hash, err := generateHash(values, deployment.Feature)
		if err != nil {
			msg := fmt.Sprintf("unable to generate feature hash: %s", err.Error())
			r.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusFailed, msg)
			continue
		}

		if ok, err := r.shouldDeployToEnvironment(ctx, deployment, environment, hash); err != nil {
			return err
		} else if !ok {
			continue
		}

		deployInstructionID, err := r.repo.DeployInstructionCreate(ctx, environment.ID, deployment.Feature, hash, &deployment.ID)
		if err != nil {
			return fmt.Errorf("create deploy instruction: %w", err)
		}

		err = mgr.Publish(ctx, message.DeployInstruction{
			ID:         deployInstructionID,
			Name:       deployment.Feature.Name,
			Version:    deployment.Feature.Version,
			Chart:      deployment.Feature.Chart,
			ConfigHash: hash,
			Timeout:    deployment.Feature.Timeout,
			Values:     values,
		})
		if err != nil {
			return fmt.Errorf("publish deploy instruction: %w", err)
		}

		r.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusCreated, "deployment instruction sent to naisd")
	}

	return nil
}

func (r *Reconciler) shouldDeployToEnvironment(ctx context.Context, deployment database.Deployment, environment *model.TenantEnvironment, hash string) (bool, error) {
	existingDeploy, err := r.repo.DeployInstructionsLatestForFeature(ctx, environment.ID, deployment.Feature.Name)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("get deploy instructions latest for environment %q: %w", environment.Name, err)
		}
	}

	if existingDeploy != nil {
		if existingDeploy.Status == model.RolloutStatusCreated || existingDeploy.Status == model.RolloutStatusPending {
			r.setDeploymentStatus(ctx, deployment.ID, environment.ID, existingDeploy.Status, "deployment is already in progress")
			return false, nil
		}

		if existingDeploy.Hash == hash {
			if existingDeploy.Status != model.RolloutStatusFailed {
				r.setDeploymentStatus(ctx, deployment.ID, environment.ID, existingDeploy.Status, "no changes in feature")
			}
			return false, nil
		}
	}

	return r.isDependenciesDeployed(ctx, deployment, environment.ID)
}

func (r *Reconciler) setDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, msg string) {
	if err := r.repo.DeploymentStatusCreateOrUpdate(ctx, deploymentID, environmentID, status, msg); err != nil {
		r.log.WithFields(logrus.Fields{
			"deployment_id":  deploymentID,
			"environment_id": environmentID,
			"status":         status,
			"msg":            msg,
		}).WithError(err).Error("create deployment status")
	}
}

func (r *Reconciler) isDependenciesDeployed(ctx context.Context, deployment database.Deployment, envID uuid.UUID) (bool, error) {
	if len(deployment.Dependencies) == 0 {
		return true, nil
	}

	for _, dep := range deployment.Dependencies {
		// TODO: the queries below assumes that a dependency is met if the feature has at any given point in time a
		// successful deployment in the environment. If this is not OK, we need to use a different table than
		// deploy_instructions to handle state stuff.

		if len(dep.AllOf) > 0 {
			missing, err := r.repo.MissingDependencies(ctx, dep.AllOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) > 0 {
				r.setDeploymentStatus(ctx, deployment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AllOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}

		if len(dep.AnyOf) > 0 {
			missing, err := r.repo.MissingDependencies(ctx, dep.AnyOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) == len(dep.AnyOf) {
				r.setDeploymentStatus(ctx, deployment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AnyOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}
	}

	return true, nil
}

// filterDeployments filters the deployments to only include the most specific deployment with the latest created
// timestamp.
func filterDeployments(deps []database.Deployment) []database.Deployment {
	deployments := map[string]database.Deployment{}
	for _, dep := range deps {
		featureName := dep.Feature.Name

		d, ok := deployments[featureName]
		if !ok {
			deployments[featureName] = dep
			continue
		}

		if len(dep.Target) > len(d.Target) {
			deployments[featureName] = dep
			continue
		}

		if len(dep.Target) == len(d.Target) && dep.Created.After(d.Created) {
			deployments[featureName] = dep
			continue
		}
	}

	ret := make([]database.Deployment, 0)
	for _, d := range deployments {
		ret = append(ret, d)
	}

	// sort by created timestamp ascending so that the oldest deployments are installed first
	slices.SortStableFunc(ret, func(a, b database.Deployment) int {
		return a.Created.Compare(b.Created)
	})

	return ret
}

func naisdTopicID(tenantName, envName string) string {
	return "naisd-" + tenantName + "-" + envName
}

func generateHash(values map[string]any, feature *model.Feature) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	b = append(b, []byte(feature.Version+feature.Chart)...)
	hash := sha256.Sum256(b)

	return hex.EncodeToString(hash[:]), nil
}
