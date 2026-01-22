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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log logrus.FieldLogger) Publisher

type deployer struct {
	publisher      NewPublisher
	repo           database.Repo
	log            logrus.FieldLogger
	deployMessages metric.Int64Counter
}

func newDeployer(
	repo database.Repo,
	publisher NewPublisher,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*deployer, error) {
	deployMessages, err := meter.Int64Counter("deployment_deploy_messages", metric.WithDescription("Deploy messages sent"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}

	return &deployer{
		publisher:      publisher,
		repo:           repo,
		log:            log,
		deployMessages: deployMessages,
	}, nil
}

func (d *deployer) deployToCI(ctx context.Context, kind model.EnvironmentKind, deploymentId uuid.UUID) error {
	tenant, err := d.repo.TenantCI(ctx)
	if err != nil {
		return fmt.Errorf("get ci tenant: %w", err)
	}
	env, err := d.repo.EnvironmentCI(ctx, kind)
	if err != nil {
		return fmt.Errorf("get ci environment for kind %q: %w", kind, err)
	}

	health, err := d.repo.HealthGet(ctx, env.ID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}

	naisdUnhealthy := time.Since(health.ReportedAt) > 3*time.Minute
	if naisdUnhealthy {
		d.log.WithFields(logrus.Fields{
			"tenant":      tenant.Name,
			"environment": env.Name,
		}).Debug("naisd is unhealthy - skip deploy to ci")
	}
	mgr := d.publisher(naisdTopicID(tenant.Name, env.Name), d.log)
	defer mgr.Stop()

	deployment, err := d.repo.V3DeploymentGet(ctx, deploymentId)
	if err != nil {
		return fmt.Errorf("get deployment %q: %w", deploymentId, err)
	}

	return d.deployToEnvironment(ctx, *deployment, &model.TenantEnvironment{
		Environment: *env,
		TenantName:  tenant.Name,
		TenantID:    tenant.ID,
	}, naisdUnhealthy, mgr)
}

func (d *deployer) deployToEnvironment(ctx context.Context, deployment database.Deployment, environment *model.TenantEnvironment, naisdUnhealthy bool, mgr Publisher) error {
	if err := d.repo.V3InsertEnvironmentFeature(ctx, environment.ID, deployment.ID, deployment.Name, deployment.Version); err != nil {
		d.log.WithError(err).WithFields(logrus.Fields{
			"environment_id":  environment.ID,
			"deployment_id":   deployment.ID,
			"feature_name":    deployment.Name,
			"feature_version": deployment.Version,
		}).Error("insert environment feature")

		d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusFailed, "failed to register feature deployment")
		return nil
	}

	if naisdUnhealthy {
		d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusPending, "naisd is unhealthy")
		return nil
	}

	values, err := d.repo.HelmValues(ctx, deployment.Feature, environment.ID)
	if err != nil {
		var fer *database.ErrMissingRequiredFields
		if errors.As(err, &fer) {
			msg := fmt.Sprintf("missing required chart config: %s", strings.Join(fer.Fields, ", "))
			d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusFailed, msg)
			return nil
		}
		return fmt.Errorf("get helm values for feature: %w", err)
	}

	hash, err := generateHash(values, deployment.Feature)
	if err != nil {
		msg := fmt.Sprintf("unable to generate feature hash: %s", err.Error())
		d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusFailed, msg)
		return nil
	}

	if ok, err := d.shouldDeployToEnvironment(ctx, deployment, environment, hash); err != nil {
		return err
	} else if !ok {
		return nil
	}

	deployInstructionID, err := d.repo.DeployInstructionCreate(ctx, environment.ID, deployment.Feature, hash, &deployment.ID)
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
	d.deployMessages.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
		attribute.String("tenant", environment.TenantName),
		attribute.String("environment", environment.Name),
		attribute.String("feature", deployment.Feature.Name),
	)))

	d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusCreated, "deployment instruction sent to naisd")
	return nil
}

func (d *deployer) shouldDeployToEnvironment(ctx context.Context, deployment database.Deployment, environment *model.TenantEnvironment, hash string) (bool, error) {
	existingDeploy, err := d.repo.DeployInstructionsLatestForFeature(ctx, environment.ID, deployment.Feature.Name)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("get deploy instructions latest for environment %q: %w", environment.Name, err)
		}
	}

	if existingDeploy != nil {
		if existingDeploy.Status == model.RolloutStatusCreated || existingDeploy.Status == model.RolloutStatusPending {
			d.setDeploymentStatus(ctx, deployment.ID, environment.ID, existingDeploy.Status, "deployment is already in progress")
			return false, nil
		}

		if existingDeploy.Hash == hash {
			return false, nil
		}
	}

	return d.isDependenciesDeployed(ctx, deployment, environment.ID)
}

func (d *deployer) setDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, msg string) {
	if err := d.repo.V3DeploymentStatusCreateOrUpdate(ctx, deploymentID, environmentID, status, msg); err != nil {
		d.log.WithFields(logrus.Fields{
			"deployment_id":  deploymentID,
			"environment_id": environmentID,
			"status":         status,
			"msg":            msg,
		}).WithError(err).Error("create deployment status")
	}
}

func (d *deployer) isDependenciesDeployed(ctx context.Context, deployment database.Deployment, envID uuid.UUID) (bool, error) {
	if len(deployment.Dependencies) == 0 {
		return true, nil
	}

	for _, dep := range deployment.Dependencies {
		// TODO: the queries below assumes that a dependency is met if the feature has at any given point in time a
		// successful deployment in the environment. If this is not OK, we need to use a different table than
		// deploy_instructions to handle state stuff.

		if len(dep.AllOf) > 0 {
			missing, err := d.repo.V3MissingDependencies(ctx, dep.AllOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) > 0 {
				d.setDeploymentStatus(ctx, deployment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AllOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}

		if len(dep.AnyOf) > 0 {
			missing, err := d.repo.V3MissingDependencies(ctx, dep.AnyOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) == len(dep.AnyOf) {
				d.setDeploymentStatus(ctx, deployment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AnyOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}
	}

	return true, nil
}

func (d *deployer) CreateDeployment(ctx context.Context, req *Request) (uuid.UUID, error) {
	feat, err := model.FromChart(req.Chart, req.Version)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to convert oci chart: %w", err)
	}

	// TODO: if we remove kind as a concept we need to change featureData table and logic
	if len(feat.EnvironmentKinds) == 0 {
		return uuid.Nil, fmt.Errorf("no environments defined in Feature.yaml")
	}

	if feat.Source == "" {
		return uuid.Nil, fmt.Errorf("no source url found in Chart.yaml")
	}

	details, err := feature.ParseTemplateDetails(feat.FeatureYAML.Values)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to parse feature template details: %w", err)
	}

	if err := d.repo.FeatureDataCreate(ctx, *feat, details); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return uuid.Nil, fmt.Errorf("unable to create feature data: %w", pgErr)
		}
	}

	deployment, err := d.repo.V3DeploymentCreate(ctx, feat.Name, req.Version, req.Description, req.Ref, req.Target)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to create deployment: %w", err)
	}

	if req.Global {
		if err := d.repo.FeatureVersionUpdate(ctx, feat.Name, req.Version); err != nil {
			return uuid.Nil, fmt.Errorf("unable to update feature version: %w", err)
		}
	}
	return deployment.ID, nil
}

// TODO: donot use database.Deployment in public methods
func (d *deployer) GetDeployment(ctx context.Context, id uuid.UUID) (*database.Deployment, error) {
	return d.repo.V3DeploymentGet(ctx, id)
}

// TODO: rename to something
func (d *deployer) deploymentsInEnvironment(ctx context.Context, environment *model.TenantEnvironment) error {
	health, err := d.repo.HealthGet(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}

	naisdUnhealthy := time.Since(health.ReportedAt) > 3*time.Minute
	if naisdUnhealthy {
		d.log.WithFields(logrus.Fields{
			"tenant":      environment.TenantName,
			"environment": environment.Name,
		}).Debug("naisd is unhealthy - skip reconcile")
	}

	mgr := d.publisher(naisdTopicID(environment.TenantName, environment.Name), d.log)
	defer mgr.Stop()

	allDeployments, err := d.repo.V3DeploymentsForEnvironment(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	for _, deployment := range filterDeployments(allDeployments) {
		if err := d.deployToEnvironment(ctx, deployment, environment, naisdUnhealthy, mgr); err != nil {
			// TODO: continue on error? earlier we returned the error immediately when the above was inside the loop.
			return err
		}
	}
	return nil
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
