package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type Reconciler struct {
	repo       *database.Repo
	featureMgr *feature.Manager
	client     *pubsub.Client
	log        *logrus.Entry
	projectID  string
}

func NewReconciler(
	repo *database.Repo,
	featureMgr *feature.Manager,
	client *pubsub.Client,
	gcpProjectID string,
	log *logrus.Entry,
) *Reconciler {
	return &Reconciler{
		repo:       repo,
		featureMgr: featureMgr,
		client:     client,
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

	envStatus, err := r.repo.StatusForEnvironment(ctx, d.ID)
	if err != nil {
		return err
	}

	lookup := make(map[string]*model.Status)
	for _, s := range envStatus {
		lookup[s.Feature] = s
	}

	mgr := message.NewPublisher[message.DeployInstruction](r.client, r.projectID, "naisd-"+d.PartnerName+"-"+d.Name, r.log)
	defer mgr.Stop()

	for _, f := range features {
		values, err := r.repo.HelmValues(ctx, f.Name, d.ID)
		if err != nil {
			return err
		}

		hash, err := generateHash(values)
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

func generateHash(values map[string]any) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
