package workers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

var rolloutConfigDescription = "updated by rollout"

type RolloutRepo interface {
	database.RolloutRepo
	database.ConfigRepo
}

type Rollout struct {
	repo database.Repo
	log  *logrus.Entry

	env    *model.Environment
	tenant *model.Tenant
}

func NewRollout(repo database.Repo, log *logrus.Entry) *Rollout {
	return &Rollout{
		repo: repo,
		log:  log,
	}
}

func (r *Rollout) Listen(ctx context.Context) error {
	return r.repo.RolloutsListen(ctx, r.process)
}

func (r *Rollout) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		r.log.Debug("checking for rollouts to process")
		if err := r.run(ctx); err != nil {
			r.log.WithError(err).Error("failed to run rollout worker")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Rollout) run(ctx context.Context) error {
	rollouts, err := r.repo.RolloutsUnprocessed(ctx)
	if err != nil {
		return err
	}

	for _, rollout := range rollouts {
		r.process(ctx, rollout.ID)
	}
	return nil
}

func (r *Rollout) process(ctx context.Context, id uuid.UUID) {
	log := r.log.WithField("id", id)
	log.Debug("got new rollout to process")

	_, env, err := r.tenantAndEnv(ctx)
	if err != nil {
		log.WithError(err).Error("failed to get tenant and environment")
		return
	}

	txRepo, tx, err := r.repo.WithTx(ctx)
	if err != nil {
		log.WithError(err).Error("failed to start transaction")
		return
	}

	rollout, err := r.getAndPrepare(ctx, txRepo, id)
	if err != nil {
		log.WithError(err).Error("failed to get rollout")
		return
	}

	var errors []error
	for k, v := range rollout.Changeset.New {
		_, err := txRepo.ConfigCreate(ctx, model.NewConfiguration{
			EnvironmentID: &env.ID,
			Feature:       rollout.Feature,
			Description:   &rolloutConfigDescription,
			Key:           k,
			Value:         v,
		})
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			log.WithField("rollout", rollout.ID).WithError(err).Error("failed to create new configurations")
		}
		tx.Rollback()
		return
	}

	tx.Commit()
}

// getAndPrepare fetches the rollout from the database and prepares it for processing.
// The main result of this function is to update the rollout with the existing values for the feature.
func (r *Rollout) getAndPrepare(ctx context.Context, repo RolloutRepo, id uuid.UUID) (*model.Rollout, error) {
	rollout, err := repo.RolloutsGet(ctx, id)
	if err != nil {
		return nil, err
	}

	if rollout.Status != model.RolloutStatusUnknown {
		return rollout, nil
	}

	_, env, err := r.tenantAndEnv(ctx)
	if err != nil {
		return rollout, err
	}

	exConfig, err := repo.ConfigGetForEnv(ctx, rollout.Feature, env.ID)
	if err != nil {
		return rollout, err
	}

	if exConfig == nil {
		return rollout, nil
	}

	lookup := map[string]json.RawMessage{}
	for _, v := range exConfig {
		lookup[v.Key] = v.Value
	}

	rollout.Changeset.Old = make(map[string]json.RawMessage)
	for k := range rollout.Changeset.New {
		rollout.Changeset.Old[k] = lookup[k]
	}

	rollout.Status = model.RolloutStatusPending

	if err := repo.RolloutUpdate(ctx, rollout); err != nil {
		return rollout, err
	}

	return rollout, nil
}

func (r *Rollout) tenantAndEnv(ctx context.Context) (*model.Tenant, *model.Environment, error) {
	var err error
	if r.tenant == nil {
		r.tenant, err = r.repo.TenantCI(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	if r.env == nil {
		r.env, err = r.repo.EnvironmentCI(ctx, model.EnvironmentKindTenant)
		if err != nil {
			return nil, nil, err
		}
	}
	return r.tenant, r.env, nil
}
