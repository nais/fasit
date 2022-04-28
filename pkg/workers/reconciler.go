package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type ReconcilerStore interface {
	TenantEnvironments(ctx context.Context) ([]*model.TenantEnvironments, error)
	StatusForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]*model.Status, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	HelmValues(ctx context.Context, feature string, envID uuid.UUID, requiredFields []string) (map[string]any, error)
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

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		r.log.Info("reconciling")
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
			"partner":     d.TenantName,
		})

		log.Debug("reconcile environment")

		if err := r.reconcileEnvironment(ctx, d); err != nil {
			log.WithError(err).Error("unable to reconcile environment")
		}
	}
	return nil
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, d *model.TenantEnvironments) error {
	features := r.featureMgr.Features[:]

	envStatus, err := r.repo.StatusForEnvironment(ctx, d.ID)
	if err != nil {
		return err
	}

	lookup := make(map[string]*model.Status)
	for _, s := range envStatus {
		lookup[s.Feature] = s
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

		values, err := r.repo.HelmValues(ctx, f.Name, d.ID, f.RequiredFields())
		if err != nil {
			var fer *database.ErrMissingRequiredFields
			if errors.As(err, &fer) {
				r.log.WithField("feature", f.Name).WithError(err).Debug("missing required fields")
				continue
			}
			return err
		}

		hash, err := generateHash(values, f)
		if err != nil {
			return err
		}

		if status, ok := lookup[f.Name]; ok {
			if status.Version == f.Version && status.ConfigHash == hash {
				continue
			}
		}

		err = mgr.Publish(ctx, message.DeployInstruction{
			Name:       f.Name,
			Version:    f.Version,
			Chart:      f.Chart,
			Repo:       f.Repo,
			ConfigHash: hash,
			Values:     values,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func generateHash(values map[string]any, feature feature.Feature) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	b = append(b, []byte(feature.Version+feature.Chart+feature.Repo)...)

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
