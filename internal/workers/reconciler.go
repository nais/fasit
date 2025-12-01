package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ReconcilerStore interface {
	DeployInstructionCreate(ctx context.Context, envID uuid.UUID, feature *model.Feature, hash string, deploymentID *uuid.UUID) (uuid.UUID, error)
	DeployInstructionsLatestForEnvironment(ctx context.Context, envID uuid.UUID) ([]*model.DeployInstruction, error)
	FeaturesForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]*model.Feature, error)
	FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *model.Feature, enabled bool) (*model.FeatureState, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	HealthGet(ctx context.Context, environmentID uuid.UUID) (*model.Health, error)
	HelmValues(ctx context.Context, feature *model.Feature, envID uuid.UUID) (map[string]any, error)
	RolloutStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	TenantEnvironments(ctx context.Context, onlyReconciled bool) ([]*model.TenantEnvironment, error)
	RolloutAssignDeployInstruction(ctx context.Context, featureName, featureVersion string, deployInstruction uuid.UUID) error
}

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log *logrus.Entry) Publisher

type Notifier interface {
	Listen(table string, filters ...notifier.Filter) <-chan notifier.Payload
}

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
	reconcileTime, err := meter.Int64Histogram("reconcile_time", metric.WithDescription("Time spent reconciling"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("unable to create reconcile_time histogram: %w", err)
	}
	deployMessages, err := meter.Int64Counter("deploy_messages", metric.WithDescription("Deploy messages sent"))
	if err != nil {
		return nil, fmt.Errorf("unable to create deploy_messages counter: %w", err)
	}

	return &Reconciler{
		repo:           repo,
		publisher:      publisher,
		log:            log,
		reconcileTime:  reconcileTime,
		deployMessages: deployMessages,
		notifier:       notifier,
	}, nil
}

func (r *Reconciler) Listen(ctx context.Context) error {
	r.log.Info("starting to listen for config changes")

	flushTimer := time.NewTicker(1 * time.Second)
	flushTimer.Stop()

	ch := make(chan struct{}, 1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				flushTimer.Reset(1 * time.Second)
			case <-flushTimer.C:
				flushTimer.Stop()
				if err := r.Reconcile(ctx); err != nil {
					r.log.WithError(err).Error("reconcile")
				}
			}
		}
	}()

	cfgGlobal := r.notifier.Listen("configurations_global")
	cfgEnv := r.notifier.Listen("configurations_environment")
	rollouts := r.notifier.Listen("rollouts", notifier.WithOperations("INSERT"))
	featureStates := r.notifier.Listen("feature_states")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-cfgGlobal:
				ch <- struct{}{}
			case <-cfgEnv:
				ch <- struct{}{}
			case <-rollouts:
				ch <- struct{}{}
			case <-featureStates:
				ch <- struct{}{}
			}
		}
	}()

	return nil
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
	ctx = auth.SetEmail(ctx, "system:reconciler")

	r.lock.Lock()

	if r.running {
		r.lock.Unlock()
		return nil
	}
	r.running = true
	r.lock.Unlock()

	defer func() {
		r.lock.Lock()
		r.running = false
		r.lock.Unlock()
	}()

	data, err := r.repo.TenantEnvironments(ctx, true)
	if err != nil {
		return err
	}

	for _, e := range data {
		log := r.log.WithFields(logrus.Fields{
			"environment": e.Name,
			"tenant":      e.TenantName,
		})

		log.Debug("reconcile environment")

		if err := r.reconcileEnvironment(ctx, e, log); err != nil {
			log.WithError(err).Error("unable to reconcile environment")
		}
	}
	return nil
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, e *model.TenantEnvironment, log *logrus.Entry) error {
	metricAttrs := []attribute.KeyValue{
		attribute.Key("environment").String(e.Name),
		attribute.Key("tenant").String(e.TenantName),
	}
	start := time.Now()
	defer func() {
		r.reconcileTime.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributes(metricAttrs...))
	}()

	health, err := r.repo.HealthGet(ctx, e.ID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}
	if time.Since(health.ReportedAt) > 3*time.Minute {
		log.Debug("naisd is unhealthy - skip reconcile")
		return nil
	}

	envInstructions, err := r.repo.DeployInstructionsLatestForEnvironment(ctx, e.Environment.ID)
	if err != nil {
		return fmt.Errorf("status for environment: %w", err)
	}

	lookup := make(map[string]*model.DeployInstruction)
	for _, ins := range envInstructions {
		lookup[ins.FeatureName] = ins
	}

	featureStates, err := r.repo.FeatureStatesGet(ctx, e.ID)
	if err != nil {
		return fmt.Errorf("feature states: %w", err)
	}

	if e.CI {
		rolloutStates, err := r.repo.RolloutStatesGet(ctx, e.ID)
		if err != nil {
			return fmt.Errorf("rollout states: %w", err)
		}

		featureStates = append(featureStates, rolloutStates...)
	}

	states := map[string]*model.FeatureState{}
	for _, s := range featureStates {
		states[s.FeatureName] = s
	}

	mgr := r.publisher(NaisdTopicID(e.TenantName, e.Name), r.log)
	defer mgr.Stop()

	features, err := r.repo.FeaturesForKind(ctx, e.Kind, e.CI)
	if err != nil {
		return fmt.Errorf("features for kind: %w", err)
	}

	for _, f := range features {
		log = log.WithField("feature", f.Name)

		if f.HasDeployments {
			log.Debug("feature is handled by deployments - skipping")
			continue
		}

		if states[f.Name] == nil || !states[f.Name].Enabled {
			continue
		}

		if l, ok := lookup[f.Name]; ok && (l.Status == model.RolloutStatusCreated || l.Status == model.RolloutStatusPending) {
			log.WithField("status", l.Status).Debug("deploy instruction already in progress")
			continue
		}

		values, err := r.repo.HelmValues(ctx, f, e.ID)
		if err != nil {
			var fer *database.ErrMissingRequiredFields
			if errors.As(err, &fer) {
				log.WithError(err).Debug("missing required fields")
				continue
			}
			return fmt.Errorf("helm values: %w", err)
		}

		hash, err := generateHash(values, f, states[f.Name].EnabledAt)
		if err != nil {
			return fmt.Errorf("generate hash: %w", err)
		}

		if status, ok := lookup[f.Name]; ok {
			if status.FeatureVersion == f.Version && status.Hash == hash {
				continue
			}
		}

		log = log.WithField("version", f.Version)
		log.Debug("publishing deploy instruction")

		r.deployMessages.Add(ctx, 1, metric.WithAttributes(append(metricAttrs, attribute.Key("feature").String(f.Name))...))

		id, err := r.repo.DeployInstructionCreate(ctx, e.ID, f, hash, nil)
		if err != nil {
			return fmt.Errorf("create deploy instruction: %w", err)
		}

		if err := r.repo.RolloutAssignDeployInstruction(ctx, f.Name, f.Version, id); err != nil {
			log.WithError(err).Error("assign deploy instruction")
		}

		err = mgr.Publish(ctx, message.DeployInstruction{
			ID:         id,
			Name:       f.Name,
			Version:    f.Version,
			Chart:      f.Chart,
			ConfigHash: hash,
			Timeout:    f.Timeout,
			Values:     values,
		})
		if err != nil {
			return fmt.Errorf("publish deploy instruction: %w", err)
		}
	}

	return nil
}

func generateHash(values map[string]any, feature *model.Feature, enabledAt *time.Time) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	at := ""
	if enabledAt != nil {
		at = enabledAt.UTC().Format(time.RFC3339)
	}

	b = append(b, []byte(feature.Version+feature.Chart+at)...)

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

func NaisdTopicID(tenantName, envName string) string {
	return "naisd-" + tenantName + "-" + envName
}
