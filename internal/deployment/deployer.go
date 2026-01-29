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
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/errgroup"
)

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log logrus.FieldLogger) Publisher

type deployerRepo interface {
	database.ConfigRepo
	database.DeployInstructionRepo
	database.EnvironmentRepo
	database.FeaturesRepo
	database.FeatureDataRepo
	database.HealthRepo
}

type deployer struct {
	publisher      NewPublisher
	repo           deployerRepo
	querier        deploymentsql.Querier
	log            logrus.FieldLogger
	deployMessages metric.Int64Counter
}

func newDeployer(repo deployerRepo, querier deploymentsql.Querier, publisher NewPublisher, meter metric.Meter, log logrus.FieldLogger) (*deployer, error) {
	deployMessages, err := meter.Int64Counter("deployment_deploy_messages", metric.WithDescription("Deploy messages sent"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}

	return &deployer{
		publisher:      publisher,
		repo:           repo,
		querier:        querier,
		log:            log,
		deployMessages: deployMessages,
	}, nil
}

func (d *deployer) naisdHealthCheck(ctx context.Context, environmentID uuid.UUID) error {
	health, err := d.repo.HealthGet(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}

	if healthy := time.Since(health.ReportedAt) <= 3*time.Minute; healthy {
		return nil
	}

	return fmt.Errorf("naisd is unhealthy")
}

func (d *deployer) deployToCI(ctx context.Context, feat *model.Feature, req Request) error {
	envs, err := d.getCIEnvironmentsForTarget(ctx, req.Target)
	if err != nil {
		return fmt.Errorf("get ci environments for target: %w", err)
	}

	deploymentsByEnvID := make(map[uuid.UUID]uuid.UUID)
	for _, env := range envs {
		var deploymentID uuid.UUID
		err := func() error {
			labels, err := d.repo.EnvironmentGetLabels(ctx, env.ID)
			if err != nil {
				return fmt.Errorf("get environment labels: %w", err)
			}

			target := make(environment.Labels)
			for _, label := range labels {
				target[label.Key] = label.Value
			}

			req.Target = target

			deploymentID, err = d.CreateDeployment(ctx, feat, req, true)
			if err != nil {
				return err
			}

			deployment, err := getDeployment(ctx, d.querier, deploymentID)
			if err != nil {
				return fmt.Errorf("get deployment %q: %w", deploymentID, err)
			}

			publisher := d.publisher(naisdTopicID(env.TenantName, env.Name), d.log)
			defer publisher.Stop()

			return d.deployToEnvironment(ctx, deployment, env, publisher)
		}()
		if err != nil {
			return err
		}

		deploymentsByEnvID[env.ID] = deploymentID
	}

	return d.waitForDeploymentStatuses(ctx, deploymentsByEnvID)
}

func (d *deployer) deployToEnvironment(ctx context.Context, deployment *model.Deployment, environment *model.TenantEnvironment, publisher Publisher) error {
	err := d.querier.InsertEnvironmentFeature(ctx, deploymentsql.InsertEnvironmentFeatureParams{
		EnvironmentID:  environment.ID,
		DeploymentID:   deployment.ID,
		FeatureName:    deployment.Feature.Name,
		FeatureVersion: deployment.Feature.Version,
	})
	if err != nil {
		d.log.WithError(err).WithFields(logrus.Fields{
			"environment_id":  environment.ID,
			"deployment_id":   deployment.ID,
			"feature_name":    deployment.Feature.Name,
			"feature_version": deployment.Feature.Version,
		}).Error("insert environment feature")

		d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusFailed, "failed to register feature deployment")
		return nil
	}

	if err := d.naisdHealthCheck(ctx, environment.ID); err != nil {
		d.setDeploymentStatus(ctx, deployment.ID, environment.ID, model.RolloutStatusPending, err.Error())
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

	err = publisher.Publish(ctx, message.DeployInstruction{
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

func (d *deployer) shouldDeployToEnvironment(ctx context.Context, deployment *model.Deployment, environment *model.TenantEnvironment, hash string) (bool, error) {
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
			if existingDeploy.Status != model.RolloutStatusFailed {
				d.setDeploymentStatus(ctx, deployment.ID, environment.ID, existingDeploy.Status, "no changes in feature")
			}
			return false, nil
		}
	}

	return d.isDependenciesDeployed(ctx, deployment, environment.ID)
}

func (d *deployer) setDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) {
	err := d.querier.DeploymentStatusCreateOrUpdate(ctx, deploymentsql.DeploymentStatusCreateOrUpdateParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
		Status:        status.String(),
		Message:       message,
	})
	if err != nil {
		d.log.WithFields(logrus.Fields{
			"deployment_id":  deploymentID,
			"environment_id": environmentID,
			"status":         status,
			"msg":            message,
		}).WithError(err).Error("create deployment status")
	}
}

func (d *deployer) isDependenciesDeployed(ctx context.Context, deployment *model.Deployment, envID uuid.UUID) (bool, error) {
	if len(deployment.Feature.Dependencies) == 0 {
		return true, nil
	}

	for _, dep := range deployment.Feature.Dependencies {
		// TODO: the queries below assumes that a dependency is met if the feature has at any given point in time a
		// successful deployment in the environment. If this is not OK, we need to use a different table than
		// deploy_instructions to handle state stuff.

		if len(dep.AllOf) > 0 {
			missing, err := d.missingDependencies(ctx, dep.AllOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) > 0 {
				d.setDeploymentStatus(ctx, deployment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AllOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}

		if len(dep.AnyOf) > 0 {
			missing, err := d.missingDependencies(ctx, dep.AnyOf, envID)
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

func (d *deployer) CreateDeployment(ctx context.Context, feat *model.Feature, req Request, ci bool) (uuid.UUID, error) {
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

	deployment, err := d.deploymentCreate(ctx, feat.Name, feat.Version, &req.Description, req.Ref, req.Target, ci)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to create deployment: %w", err)
	}

	if req.Global {
		if err := d.repo.FeatureVersionUpdate(ctx, feat.Name, feat.Version); err != nil {
			return uuid.Nil, fmt.Errorf("unable to update feature version: %w", err)
		}
	}
	return deployment.ID, nil
}

func (d *deployer) waitForDeploymentStatuses(ctx context.Context, deploymentsByEnvID map[uuid.UUID]uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	for envID, deploymentID := range deploymentsByEnvID {
		eg.Go(func() error {
			for {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				status, err := d.querier.LatestStatusForDeploymentInEnvironment(ctx, deploymentsql.LatestStatusForDeploymentInEnvironmentParams{
					DeploymentID:  deploymentID,
					EnvironmentID: envID,
				})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("get latest deployment status for deployment %q in environment %q: %w", deploymentID, envID, err)
				}

				state := model.DeploymentStatusState(strings.ToUpper(status))

				switch state {
				case model.DeploymentStatusStateDeployed:
					return nil
				case model.DeploymentStatusStateFailed:
					return fmt.Errorf("deployment %q in environment %q failed", deploymentID, envID)
				}

				select {
				case <-ctx.Done():
					return fmt.Errorf("timeout waiting for deployment %q in environment %q to complete", deploymentID, envID)
				case <-time.After(5 * time.Second):
				}
			}
		})
	}

	return eg.Wait()
}

func (d *deployer) getCIEnvironmentsForTarget(ctx context.Context, labels environment.Labels) ([]*model.TenantEnvironment, error) {
	envs, err := d.querier.GetCIEnvironmentsForTarget(ctx, labels)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.TenantEnvironment, len(envs))
	for i, e := range envs {
		ret[i] = &model.TenantEnvironment{
			Environment: model.Environment{
				ID:           e.Environment.ID,
				Name:         e.Environment.Name,
				CI:           e.Environment.Ci,
				Description:  e.Environment.Description,
				Created:      e.Environment.Created.Time,
				LastModified: e.Environment.LastModified.Time,
				Kind:         model.EnvironmentKind(e.Environment.Kind),
			},
			TenantName: e.TenantName,
			TenantID:   e.Environment.TenantID,
		}
	}
	return ret, nil
}

func (d *deployer) missingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error) {
	if len(dependencies) == 0 {
		return []string{}, nil
	}
	deployedFeatures, err := d.querier.DeployInstructionsGetDeployedFeatures(ctx, deploymentsql.DeployInstructionsGetDeployedFeaturesParams{
		FeatureNames:  dependencies,
		EnvironmentID: environmentID,
	})
	if err != nil {
		return nil, err
	}

	missing := make([]string, 0)
	for _, d := range dependencies {
		if !slices.Contains(deployedFeatures, d) {
			missing = append(missing, d)
		}
	}
	return missing, nil
}

func (d *deployer) deploymentCreate(ctx context.Context, featureName, featureVersion string, description *string, ref *model.GHRef, target environment.Labels, ci bool) (*deploymentsql.Deployment, error) {
	var ghRef []byte
	if ref != nil {
		b, err := json.Marshal(ref)
		if err != nil {
			return nil, fmt.Errorf("marshal gh ref: %w", err)
		}

		ghRef = b
	}

	ret, err := d.querier.DeploymentCreate(ctx, deploymentsql.DeploymentCreateParams{
		FeatureName: featureName,
		Version:     featureVersion,
		GhRef:       ghRef,
		Target:      target,
		Description: description,
		Ci:          ci,
	})

	return &ret, err
}

// filterDeployments filters the deployments to only include the most specific deployment with the latest created
// timestamp.
func filterDeployments(deps []*model.Deployment) []*model.Deployment {
	deployments := map[string]*model.Deployment{}
	for _, dep := range deps {
		featureName := dep.Feature.Name

		d, ok := deployments[featureName]
		if !ok {
			deployments[featureName] = dep
			continue
		}

		if len(dep.TargetLabels) > len(d.TargetLabels) {
			deployments[featureName] = dep
			continue
		}

		if len(dep.TargetLabels) == len(d.TargetLabels) && dep.Created.After(d.Created) {
			deployments[featureName] = dep
			continue
		}
	}

	ret := make([]*model.Deployment, 0)
	for _, d := range deployments {
		ret = append(ret, d)
	}

	// sort by created timestamp ascending so that the oldest deployments are installed first
	slices.SortStableFunc(ret, func(a, b *model.Deployment) int {
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
