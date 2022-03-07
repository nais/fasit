package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

type Reconciler struct {
	repo       *database.Repo
	featureMgr *feature.Manager
	log        *logrus.Entry
}

func NewReconciler(repo *database.Repo, featureMgr *feature.Manager, log *logrus.Entry) *Reconciler {
	return &Reconciler{
		repo:       repo,
		featureMgr: featureMgr,
		log:        log,
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
	data, err := r.repo.ReconcileData(ctx)
	if err != nil {
		return err
	}

	for _, d := range data {
		if err := r.reconcileEnvironment(ctx, d); err != nil {
			r.log.WithError(err).
				WithFields(logrus.Fields{
					"environment": d.Name,
					"partner":     d.PartnerName,
				}).
				Error("unable to reconcile environment")
		}
	}
	return nil
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, d *model.ReconcileData) error {
	features := r.featureMgr.Features[:]

	status, err := r.repo.StatusForEnvironment(ctx, d.ID)
	if err != nil {
		return err
	}

	lookup := make(map[string]*model.Status)
	for _, s := range status {
		lookup[s.Feature] = s
	}

	for _, f := range features {
		values, err := r.repo.HelmValues(ctx, f.Name, d.ID)
		if err != nil {
			return err
		}

		if status, ok := lookup[f.Name]; ok {

			hash, err := generateHash(values)
			if err != nil {
				return err
			}

			if status.Version == f.Version && status.ConfigHash == hash {
				continue
			}

			// Check if the configuration has changed
			// if so, continue to next feature
		}

		// Implment whatever the naisd boys create in the other room
		// pubsub.PublishHelmChart()
		hash, _ := generateHash(values)
		r.log.WithFields(logrus.Fields{
			"partner":         d.PartnerName,
			"environment":     d.Name,
			"config_hash":     hash,
			"feature":         f.Name,
			"feature_version": f.Version,
		}).Info("rollout")

	}
	return nil
}

func generateHash(values map[string]any) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
