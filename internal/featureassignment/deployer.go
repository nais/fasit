package featureassignment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	commonmodel "github.com/nais/fasit/internal/model"
	"github.com/nais/fasit/internal/naisdstatus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log *slog.Logger) Publisher

type deployer struct {
	newPublisher   NewPublisher
	querier        featureassignmentsql.Querier
	pool           *pgxpool.Pool
	log            *slog.Logger
	deployMessages metric.Int64Counter

	publishersMu sync.Mutex
	publishers   map[string]Publisher
}

func newDeployer(pool *pgxpool.Pool, querier featureassignmentsql.Querier, publisher NewPublisher, meter metric.Meter, log *slog.Logger) (*deployer, error) {
	deployMessages, err := meter.Int64Counter("assignment_deploy_messages", metric.WithDescription("Deploy messages sent"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}

	return &deployer{
		newPublisher:   publisher,
		querier:        querier,
		pool:           pool,
		log:            log,
		deployMessages: deployMessages,
		publishers:     make(map[string]Publisher),
	}, nil
}

func (d *deployer) publisher(topicID string) Publisher {
	d.publishersMu.Lock()
	defer d.publishersMu.Unlock()
	if p, ok := d.publishers[topicID]; ok {
		return p
	}
	p := d.newPublisher(topicID, d.log)
	d.publishers[topicID] = p
	return p
}

func (d *deployer) naisdHealthCheck(ctx context.Context, environmentID uuid.UUID) error {
	health, err := naisdstatus.Get(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}

	if healthy := time.Since(health.ReportedAt) <= 3*time.Minute; healthy {
		return nil
	}

	return fmt.Errorf("naisd is unhealthy")
}

func (d *deployer) deployToEnvironment(ctx context.Context, assignment *FeatureAssignment, environment *model.TenantEnvironment, publisher Publisher) error {
	if err := d.naisdHealthCheck(ctx, environment.ID); err != nil {
		d.setFeatureReconcileStatus(ctx, assignment.ID, environment.ID, model.RolloutStatusPending, err.Error())
		return nil
	}

	values, err := featurepkg.HelmValues(ctx, assignment.Feature, environment.ID)
	if err != nil {
		var fer *errs.ErrMissingRequiredFields
		if errors.As(err, &fer) {
			msg := fmt.Sprintf("missing required chart config: %s", strings.Join(fer.Fields, ", "))
			d.setFeatureReconcileStatus(ctx, assignment.ID, environment.ID, model.RolloutStatusFailed, msg)
			return nil
		}
		return fmt.Errorf("get helm values for feature: %w", err)
	}

	hash, err := generateHash(values, assignment.Feature)
	if err != nil {
		msg := fmt.Sprintf("unable to generate feature hash: %s", err.Error())
		d.setFeatureReconcileStatus(ctx, assignment.ID, environment.ID, model.RolloutStatusFailed, msg)
		return nil
	}

	if ok, err := d.shouldDeployToEnvironment(ctx, assignment, environment, hash); err != nil {
		return err
	} else if !ok {
		return nil
	}

	deployInstructionID, err := d.createDeployInstruction(ctx, environment.ID, assignment.Feature, hash, &assignment.ID)
	if err != nil {
		return fmt.Errorf("create deploy instruction: %w", err)
	}

	err = publisher.Publish(ctx, message.DeployInstruction{
		ID:         deployInstructionID,
		Name:       assignment.Feature.Name,
		Version:    assignment.Feature.Version,
		Chart:      assignment.Feature.Chart,
		ConfigHash: hash,
		Timeout:    assignment.Feature.Timeout,
		Values:     values,
	})
	if err != nil {
		return fmt.Errorf("publish deploy instruction: %w", err)
	}
	d.deployMessages.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
		attribute.String("tenant", environment.TenantName),
		attribute.String("environment", environment.Name),
		attribute.String("feature", assignment.Feature.Name),
	)))

	d.setFeatureReconcileStatus(ctx, assignment.ID, environment.ID, model.RolloutStatusCreated, "assign instruction sent to naisd")
	return nil
}

func (d *deployer) shouldDeployToEnvironment(ctx context.Context, assignment *FeatureAssignment, environment *model.TenantEnvironment, hash string) (bool, error) {
	existingDeploy, err := featurepkg.GetLatestDeployInstruction(ctx, environment.ID, assignment.Feature.Name)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("get deploy instructions latest for environment %q: %w", environment.Name, err)
		}
	}

	if existingDeploy != nil {
		if existingDeploy.Status == model.RolloutStatusCreated || existingDeploy.Status == model.RolloutStatusPending {
			d.setFeatureReconcileStatus(ctx, assignment.ID, environment.ID, existingDeploy.Status, "assignment is already in progress")
			return false, nil
		}

		if existingDeploy.Hash == hash {
			if existingDeploy.Status != model.RolloutStatusFailed {
				d.setFeatureReconcileStatus(ctx, assignment.ID, environment.ID, existingDeploy.Status, "no changes in feature")
			}
			return false, nil
		}
	}

	return d.isDependenciesDeployed(ctx, assignment, environment.ID)
}

func (d *deployer) setFeatureReconcileStatus(ctx context.Context, featureAssignmentID, environmentID uuid.UUID, status model.RolloutStatus, message string) {
	err := d.querier.SetReconcileStatus(ctx, featureassignmentsql.SetReconcileStatusParams{
		FeatureAssignmentID: featureAssignmentID,
		EnvironmentID:       environmentID,
		Status:              status.String(),
		Message:             message,
	})
	if err != nil {
		d.log.With(
			"err", err,
			"feature_assignment_id", featureAssignmentID,
			"environment_id", environmentID,
			"status", status,
			"msg", message,
		).Error("create feature assignment status")
	}
}

func (d *deployer) isDependenciesDeployed(ctx context.Context, assignment *FeatureAssignment, envID uuid.UUID) (bool, error) {
	if len(assignment.Feature.Dependencies) == 0 {
		return true, nil
	}

	for _, dep := range assignment.Feature.Dependencies {
		// TODO: the queries below assumes that a dependency is met if the feature has at any given point in time a
		// successful deployment in the environment. If this is not OK, we need to use a different table than
		// deploy_instructions to handle state stuff.

		if len(dep.AllOf) > 0 {
			missing, err := d.missingDependencies(ctx, dep.AllOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) > 0 {
				d.setFeatureReconcileStatus(ctx, assignment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AllOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}

		if len(dep.AnyOf) > 0 {
			missing, err := d.missingDependencies(ctx, dep.AnyOf, envID)
			if err != nil {
				return false, fmt.Errorf("get features not in env: %w", err)
			}

			if len(missing) == len(dep.AnyOf) {
				d.setFeatureReconcileStatus(ctx, assignment.ID, envID, model.RolloutStatusFailed, "missing dependencies (AnyOf): "+strings.Join(missing, ", "))
				return false, nil
			}
		}
	}

	return true, nil
}

func (d *deployer) CreateFeatureAssignment(ctx context.Context, feat *model.Feature, description *string, githubRef *commonmodel.GitHubCommit, target environment.Labels) (uuid.UUID, error) {
	details, err := featurepkg.ParseTemplateDetails(feat.Values)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to parse feature template details: %w", err)
	}

	if err := featurepkg.FeatureDataCreate(ctx, *feat, details); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return uuid.Nil, fmt.Errorf("unable to create feature data: %w", pgErr)
		}
	}

	var ghRef []byte
	if githubRef != nil {
		b, err := json.Marshal(githubRef.Ref)
		if err != nil {
			return uuid.Nil, fmt.Errorf("marshal gh ref: %w", err)
		}

		ghRef = b
	}

	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQuerier := featureassignmentsql.New(tx)

	err = txQuerier.DeactivateActiveFeatureAssignmentForTarget(ctx, featureassignmentsql.DeactivateActiveFeatureAssignmentForTargetParams{
		FeatureName: feat.Name,
		Target:      types.EnvironmentLabels(target),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("deactivate previous assignment: %w", err)
	}

	assignment, err := txQuerier.CreateFeatureAssignment(ctx, featureassignmentsql.CreateFeatureAssignmentParams{
		FeatureName: feat.Name,
		Version:     feat.Version,
		GhRef:       ghRef,
		Target:      types.EnvironmentLabels(target),
		Description: description,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to create feature assignment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit tx: %w", err)
	}

	return assignment.ID, nil
}

func (d *deployer) missingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error) {
	if len(dependencies) == 0 {
		return []string{}, nil
	}
	deployedFeatures, err := d.querier.ListDeployedFeaturesInEnvironment(ctx, featureassignmentsql.ListDeployedFeaturesInEnvironmentParams{
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

func (d *deployer) createDeployInstruction(ctx context.Context, envID uuid.UUID, feature *model.Feature, hash string, featureAssignmentID *uuid.UUID) (uuid.UUID, error) {
	vals, err := featurepkg.HelmValues(ctx, feature, envID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get helm values: %w", err)
	}

	values, err := json.Marshal(vals)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal helm values: %w", err)
	}

	return d.querier.CreateDeployInstruction(ctx, featureassignmentsql.CreateDeployInstructionParams{
		EnvironmentID:       envID,
		FeatureName:         feature.Name,
		FeatureVersion:      feature.Version,
		Hash:                hash,
		Values:              values,
		FeatureAssignmentID: featureAssignmentID,
	})
}

// IsMoreSpecific reports whether a deployment with candidateLabels (created at
// candidateCreated) should replace one with existingLabels (created at
// existingCreated). More target labels means more specific. Equal count: latest wins.
func IsMoreSpecific(candidateLabels, existingLabels map[string]string, candidateCreated, existingCreated time.Time) bool {
	if len(candidateLabels) > len(existingLabels) {
		return true
	}
	return len(candidateLabels) == len(existingLabels) && candidateCreated.After(existingCreated)
}

// mostSpecificPerFeature picks one feature assignment per feature name: the one with
// the most specific target labels (most labels wins), breaking ties by latest
// created timestamp.
func mostSpecificPerFeature(deps []*FeatureAssignment) []*FeatureAssignment {
	assignments := map[string]*FeatureAssignment{}
	for _, dep := range deps {
		existing, ok := assignments[dep.Feature.Name]
		if !ok || IsMoreSpecific(dep.TargetLabels, existing.TargetLabels, dep.Created, existing.Created) {
			assignments[dep.Feature.Name] = dep
		}
	}

	ret := make([]*FeatureAssignment, 0)
	for _, d := range assignments {
		ret = append(ret, d)
	}

	slices.SortStableFunc(ret, func(a, b *FeatureAssignment) int {
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
