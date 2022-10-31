package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type ReconcilerStore interface {
	ConfigListen(ctx context.Context, fn database.ListenFunc) error
	FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *feature.Feature, enabled bool) (*model.FeatureState, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	HealthGet(ctx context.Context, environmentID uuid.UUID) (*model.Health, error)
	HelmValues(ctx context.Context, feature feature.Feature, envID uuid.UUID) (map[string]any, []uuid.UUID, error)
	RolloutEventCreate(ctx context.Context, event *model.RolloutEvent) error
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
}

func NewReconciler(
	repo ReconcilerStore,
	featureMgr *feature.Manager,
	publisher NewPublisher,
	gcpProjectID string,
	log *logrus.Entry,
) *Reconciler {
	return &Reconciler{
		repo:       repo,
		featureMgr: featureMgr,
		publisher:  publisher,
		log:        log,
		projectID:  gcpProjectID,
	}
}

func (r *Reconciler) Listen(ctx context.Context) error {
	r.log.Info("starting to listen for config changes")

	flushTimer := time.NewTicker(1 * time.Second)
	flushTimer.Stop()

	ch := make(chan uuid.UUID, 1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				flushTimer.Reset(1 * time.Second)
			case <-flushTimer.C:
				r.reconcile(ctx)
			}
		}
	}()

	return r.repo.ConfigListen(ctx, func(ctx context.Context, envID uuid.UUID) {
		ch <- envID
	})
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

func (r *Reconciler) autoInstallNextFeature(ctx context.Context, d *model.TenantEnvironments, features []feature.Feature, status map[string]*model.Status) error {
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

		// Dependency not enabled
		if len(f.DependsOn.FindMissing(enabledFeatures)) > 0 {
			continue
		}

		_, err := r.repo.FeatureStatesCreateOrUpdate(ctx, d.ID, &f, true)
		if err != nil {
			return fmt.Errorf("unable to enable feature %s: %w", f.Name, err)
		}
		return nil
	}
	return nil
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, d *model.TenantEnvironments) error {
	health, err := r.repo.HealthGet(ctx, d.ID)
	if err != nil {
		return err
	}
	if time.Since(health.ReportedAt) > 3*time.Minute {
		r.log.WithField("environment", d.ID).Infof("naisd is unhealthy - skip reconcile")
		return nil
	}

	features := r.featureMgr.Features[:]

	envStatus, err := r.repo.StatusForEnvironment(ctx, d.ID)
	if err != nil {
		return err
	}

	lookup := make(map[string]*model.Status)
	for _, s := range envStatus {
		lookup[s.Feature] = s
	}

	err = r.autoInstallNextFeature(ctx, d, features, lookup)
	if err != nil {
		r.log.WithField("environment", d.Environment.ID).WithError(err).Errorf("unable to auto enable feature")
	}

	mgr := r.publisher(r.projectID, "naisd-"+d.TenantName+"-"+d.Name, r.log)
	defer mgr.Stop()

	featureStates, err := r.repo.FeatureStatesGet(ctx, d.ID)
	if err != nil {
		return err
	}

	states := map[string]*model.FeatureState{}
	for _, s := range featureStates {
		states[s.FeatureName] = s
	}

	for _, f := range features {
		if states[f.Name] == nil || !states[f.Name].Enabled {
			r.log.WithField("feature", f.Name).Debug("not enabled")
			continue
		}

		values, rolloutIDs, err := r.repo.HelmValues(ctx, f, d.ID)
		if err != nil {
			var fer *database.ErrMissingRequiredFields
			if errors.As(err, &fer) {
				r.log.WithField("feature", f.Name).WithError(err).Info("missing required fields")
				continue
			}
			return err
		}

		createRolloutEvent := func(typ model.RolloutEventType, data map[string]any) {
			var (
				b   []byte
				err error
			)

			if data != nil {
				b, err = json.Marshal(data)
				if err != nil {
					r.log.WithError(err).Error("unable to marshal rollout event data")
					return
				}
			}
			for _, rid := range rolloutIDs {
				_ = r.repo.RolloutEventCreate(ctx, &model.RolloutEvent{
					RolloutID: rid,
					Type:      typ,
					Data:      b,
				})
			}
		}

		createRolloutEvent(model.RolloutEventTypeInProgress, nil)

		hash, err := generateHash(values, f, states[f.Name].EnabledAt)
		if err != nil {
			createRolloutEvent(model.RolloutEventTypeInProgress, map[string]any{
				"error": err.Error(),
			})

			return err
		}

		if status, ok := lookup[f.Name]; ok {
			if status.Version == f.Version && status.ConfigHash == hash {
				createRolloutEvent(model.RolloutEventTypeFailed, map[string]any{
					"error": "no changes",
				})
				continue
			}
		}

		r.log.WithFields(logrus.Fields{
			"feature":     f.Name,
			"tenant":      d.TenantName,
			"environment": d.Name,
		}).Info("publish deploy instruction")

		err = mgr.Publish(ctx, message.DeployInstruction{
			Name:       f.Name,
			Version:    f.Version,
			Chart:      f.Chart,
			Repo:       f.Repo,
			ConfigHash: hash,
			Timeout:    f.Timeout,
			Values:     values,
			RolloutIDs: rolloutIDs,
		})
		if err != nil {
			createRolloutEvent(model.RolloutEventTypeFailed, map[string]any{
				"error": err.Error(),
			})
			return err
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
