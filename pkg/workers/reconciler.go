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
	"github.com/nais/fasit/pkg/auth"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

type ReconcilerStore interface {
	ConfigListen(ctx context.Context, fn database.ListenFunc) error
	FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *feature.Feature, enabled bool) (*model.FeatureState, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	FeatureStatesListen(ctx context.Context, fn database.ListenFunc) error
	HealthGet(ctx context.Context, environmentID uuid.UUID) (*model.Health, error)
	HelmValues(ctx context.Context, feature feature.Feature, envID uuid.UUID) (map[string]any, error)
	RolloutsListen(ctx context.Context, fn database.ListenFunc) error
	StatusForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]*model.Status, error)
	TenantEnvironments(ctx context.Context) ([]*model.TenantEnvironments, error)
}

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(projectID, topicID string, log *logrus.Entry) Publisher

type Reconciler struct {
	repo       ReconcilerStore
	featureMgr *feature.Manager
	publisher  NewPublisher
	log        *logrus.Entry
	projectID  string

	lock    sync.Mutex
	running bool

	// Metrics
	reconcileTime  syncint64.Histogram
	deployMessages syncint64.Counter
}

func NewReconciler(
	repo ReconcilerStore,
	featureMgr *feature.Manager,
	publisher NewPublisher,
	gcpProjectID string,
	meter metric.Meter,
	log *logrus.Entry,
) (*Reconciler, error) {
	reconcileTime, err := meter.Int64Histogram("reconcile_time", instrument.WithDescription("Time spent reconciling"), instrument.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("unable to create reconcile_time histogram: %w", err)
	}
	deployMessages, err := meter.Int64Counter("deploy_messages", instrument.WithDescription("Deploy messages sent"))
	if err != nil {
		return nil, fmt.Errorf("unable to create deploy_messages counter: %w", err)
	}

	return &Reconciler{
		repo:           repo,
		featureMgr:     featureMgr,
		publisher:      publisher,
		log:            log,
		projectID:      gcpProjectID,
		reconcileTime:  reconcileTime,
		deployMessages: deployMessages,
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
				r.reconcile(ctx)
			}
		}
	}()

	trigger := func(ctx context.Context, id uuid.UUID) {
		ch <- struct{}{}
	}

	go func() {
		if err := r.repo.RolloutsListen(ctx, trigger); err != nil {
			r.log.WithError(err).Error("rollouts listen")
		}
	}()

	go func() {
		if err := r.repo.FeatureStatesListen(ctx, trigger); err != nil {
			r.log.WithError(err).Error("feature states listen")
		}
	}()

	return r.repo.ConfigListen(ctx, trigger)
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		r.log.Debug("reconciling")
		if err := r.reconcile(ctx); err != nil {
			r.log.WithError(err).Error("reconcile")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Reconciler) reconcile(ctx context.Context) error {
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

	data, err := r.repo.TenantEnvironments(ctx)
	if err != nil {
		return err
	}

	for _, d := range data {
		log := r.log.WithFields(logrus.Fields{
			"environment": d.Name,
			"tenant":      d.TenantName,
		})

		log.Debug("reconcile environment")

		if err := r.reconcileEnvironment(ctx, d); err != nil {
			log.WithError(err).Error("unable to reconcile environment")
		}
	}
	return nil
}

func (r *Reconciler) autoInstallNextFeature(
	ctx context.Context,
	d *model.TenantEnvironments,
	features []feature.Feature,
	status map[string]*model.Status,
	featureStates map[string]*model.FeatureState,
) error {
	r.log.Debug("Auto install next feature")
	enabledFeatures := []string{}
	for _, s := range status {
		if s.Status == model.RolloutStatusDeployed {
			enabledFeatures = append(enabledFeatures, s.Feature)
		}
	}

	for _, f := range features {
		if !contains(f.AutoInstall, d.Kind) {
			continue
		}

		// Feature already enabled and rolled out to environment successfully
		if s, ok := status[f.Name]; ok && s.Status == model.RolloutStatusDeployed {
			continue
		} else if ok {
			// Feature already enabled but not yet deployed to environment
			break
		}

		if _, ok := featureStates[f.Name]; ok {
			r.log.WithField("feature", f.Name).Info("feature state already exists, skipping auto install for environment")
			return nil
		}

		// Dependency not enabled
		if len(f.DependsOn.FindMissing(enabledFeatures)) > 0 {
			continue
		}

		r.log.WithField("feature", f.Name).Info("Auto install feature")
		_, err := r.repo.FeatureStatesCreateOrUpdate(ctx, d.ID, &f, true)
		if err != nil {
			return fmt.Errorf("unable to enable feature %s: %w", f.Name, err)
		}
		return nil
	}
	return nil
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, d *model.TenantEnvironments) error {
	metricAttrs := []attribute.KeyValue{
		attribute.Key("environment").String(d.Name),
		attribute.Key("tenant").String(d.TenantName),
	}
	start := time.Now()
	defer func() {
		r.reconcileTime.Record(ctx, time.Since(start).Milliseconds(), metricAttrs...)
	}()

	health, err := r.repo.HealthGet(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("health status: %w", err)
	}
	if time.Since(health.ReportedAt) > 3*time.Minute {
		r.log.WithField("environment", d.ID).Infof("naisd is unhealthy - skip reconcile")
		return nil
	}

	features := r.featureMgr.Features()

	envStatus, err := r.repo.StatusForEnvironment(ctx, d.Environment.ID)
	if err != nil {
		return fmt.Errorf("status for environment: %w", err)
	}

	lookup := make(map[string]*model.Status)
	for _, s := range envStatus {
		lookup[s.Feature] = s
	}

	featureStates, err := r.repo.FeatureStatesGet(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("feature states: %w", err)
	}

	states := map[string]*model.FeatureState{}
	for _, s := range featureStates {
		states[s.FeatureName] = s
	}

	err = r.autoInstallNextFeature(ctx, d, features, lookup, states)
	if err != nil {
		r.log.WithField("environment", d.Environment.ID).WithError(err).Errorf("unable to auto enable feature")
	}

	mgr := r.publisher(r.projectID, "naisd-"+d.TenantName+"-"+d.Name, r.log)
	defer mgr.Stop()

	for _, f := range features {
		if states[f.Name] == nil || !states[f.Name].Enabled {
			// r.log.WithField("feature", f.Name).Debug("not enabled")
			continue
		}

		values, err := r.repo.HelmValues(ctx, f, d.ID)
		if err != nil {
			var fer *database.ErrMissingRequiredFields
			if errors.As(err, &fer) {
				r.log.WithField("feature", f.Name).WithError(err).Info("missing required fields")
				continue
			}
			return fmt.Errorf("helm values: %w", err)
		}

		hash, err := generateHash(values, f, states[f.Name].EnabledAt)
		if err != nil {
			return fmt.Errorf("generate hash: %w", err)
		}

		if status, ok := lookup[f.Name]; ok {
			if status.Version == f.Version && status.ConfigHash == hash {
				continue
			}
		}

		r.log.WithFields(logrus.Fields{
			"feature":     f.Name,
			"tenant":      d.TenantName,
			"environment": d.Name,
		}).Info("publish deploy instruction")

		r.deployMessages.Add(ctx, 1, append(metricAttrs, attribute.Key("feature").String(f.Name))...)
		err = mgr.Publish(ctx, message.DeployInstruction{
			Name:       f.Name,
			Version:    f.Version,
			Chart:      f.Chart,
			Repo:       f.Repo,
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

func generateHash(values map[string]any, feature feature.Feature, enabledAt *time.Time) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	at := ""
	if enabledAt != nil {
		at = enabledAt.UTC().Format(time.RFC3339)
	}

	b = append(b, []byte(feature.Version+feature.Chart+feature.Repo+at)...)

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

func contains[T comparable](a []T, x T) bool {
	for _, n := range a {
		if n == x {
			return true
		}
	}
	return false
}
