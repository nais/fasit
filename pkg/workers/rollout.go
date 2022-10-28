package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

type RolloutRepo interface {
	database.RolloutRepo
	database.ConfigRepo
}

type Rollout struct {
	repo database.Repo
	log  *logrus.Entry
}

func NewRollout(repo database.Repo, log *logrus.Entry) *Rollout {
	return &Rollout{
		repo: repo,
		log:  log,
	}
}

func (r *Rollout) Listen(ctx context.Context) error {
	r.log.Info("started listener for rollout events")
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

func (r *Rollout) Notify(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error {
	switch status {
	case model.RolloutStatusDeployed:
		if err := r.completeRolloutEvent(ctx, id); err != nil {
			r.addErrorEvent(ctx, id, err)
			updateErr := r.repo.RolloutsUpdateStatus(ctx, []uuid.UUID{id}, model.RolloutStatusFailed)
			if updateErr != nil {
				r.log.WithError(updateErr).Error("failed to update status on failed rollout")
			}
			return err
		}

		r.addEvent(ctx, id, model.RolloutEventTypeSuccess, nil)
		return nil
	case model.RolloutStatusFailed:
		if err := r.rollback(ctx, id); err != nil {
			r.addErrorEvent(ctx, id, err)
			return err
		}
		r.addEvent(ctx, id, model.RolloutEventTypeRolledBack, nil)
		return nil
	default:
		return nil
	}
}

func (r *Rollout) completeRolloutEvent(ctx context.Context, rolloutID uuid.UUID) error {
	log := r.log.WithField("id", rolloutID)
	rollout, err := r.repo.RolloutGetByID(ctx, rolloutID)
	if err != nil {
		log.WithError(err).Error("failed to get rollout")
		return err
	}

	done, err := r.repo.RolloutSummaryDone(ctx, rollout.RolloutSummaryID)
	if err != nil {
		log.WithError(err).Error("failed to get rollout summary")
		return err
	}

	if err := r.markRolloutDeployed(ctx, rolloutID); err != nil {
		log.WithError(err).Error("failed to mark rollout as deployed")
		return err
	}

	if !done {
		log.Warn("rollout summary not done")
	}

	txRepo, tx, err := r.repo.WithTx(ctx)
	if err != nil {
		log.WithError(err).Error("failed to start transaction")
		return err
	}

	if err := txRepo.ConfigDeleteByRolloutID(ctx, rollout.ID); err != nil {
		log.WithError(err).Error("failed to delete rollout configurations")
		tx.Rollback(ctx)
		return err
	}

	for k, v := range rollout.Changeset.New {
		_, err := txRepo.ConfigCreate(ctx, model.NewConfiguration{
			Feature: rollout.Feature,
			Key:     k,
			Value:   v,
		})
		if err != nil {
			log.WithError(err).Error("failed to create new global configuration")
			tx.Rollback(ctx)
			return err
		}
	}

	rollout.Status = model.RolloutStatusDeployed
	if err := txRepo.RolloutUpdate(ctx, rollout); err != nil {
		log.WithError(err).Error("failed to update rollout")
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (r *Rollout) markRolloutDeployed(ctx context.Context, rolloutID uuid.UUID) error {
	log := r.log.WithField("id", rolloutID)
	log.Debug("marking rollout as deployed")

	return r.repo.RolloutsUpdateStatus(ctx, []uuid.UUID{rolloutID}, model.RolloutStatusDeployed)
}

func (r *Rollout) rollback(ctx context.Context, id uuid.UUID) error {
	log := r.log.WithField("id", id)
	log.Debug("rolling back rollout")

	rollout, err := r.repo.RolloutGetByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to get rollout")
		return err
	}

	if err := r.repo.RolloutsUpdateStatus(ctx, []uuid.UUID{rollout.ID}, model.RolloutStatusFailed); err != nil {
		log.WithError(err).Error("failed to set rollout status to failed")
		return err
	}

	_, env, err := r.tenantAndEnv(ctx, rollout.EnvironmentKind)
	if err != nil {
		return err
	}

	configs, err := r.repo.ConfigGetForEnv(ctx, rollout.Feature, env.ID)
	if err != nil {
		log.WithError(err).Error("failed to get configurations for environment")
		return err
	}

	getConfig := func(key string) *model.EnvConfiguration {
		for _, config := range configs {
			if config.Key == key {
				return config
			}
		}
		return nil
	}

	txRepo, tx, err := r.repo.WithTx(ctx)
	if err != nil {
		log.WithError(err).Error("failed to start transaction")
		return err
	}
	var errors []error
	for k, v := range rollout.Changeset.Old {
		if v == nil || bytes.EqualFold(v, []byte("null")) {
			if c := getConfig(k); c != nil {
				if err := txRepo.ConfigDelete(ctx, c.ID); err != nil {
					errors = append(errors, err)
				}
			}
		} else {
			_, err := txRepo.ConfigCreate(ctx, model.NewConfiguration{
				Feature:       rollout.Feature,
				Key:           k,
				Value:         v,
				EnvironmentID: &env.ID,
			})
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			log.WithField("rollout", rollout.ID).WithError(err).Error("failed to delete new configurations")
		}
		tx.Rollback(ctx)
		return fmt.Errorf("error while rolling back rollout: %v", errors)
	}

	return tx.Commit(ctx)
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

	txRepo, tx, err := r.repo.WithTx(ctx)
	if err != nil {
		log.WithError(err).Error("failed to start transaction")
		r.addErrorEvent(ctx, id, err)
		return
	}

	rollout, err := r.getAndPrepare(ctx, txRepo, id)
	if err != nil {
		log.WithError(err).Error("failed to get rollout")
		r.addErrorEvent(ctx, id, err)
		return
	}

	_, env, err := r.tenantAndEnv(ctx, rollout.EnvironmentKind)
	if err != nil {
		log.WithError(err).Error("failed to get tenant and environment")
		r.addErrorEvent(ctx, id, err)
		return
	}

	var errors []error
	for k, v := range rollout.Changeset.New {
		_, err := txRepo.ConfigCreate(ctx, model.NewConfiguration{
			EnvironmentID: &env.ID,
			Feature:       rollout.Feature,
			Key:           k,
			Value:         v,
			RolloutID:     &rollout.ID,
		})
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			log.WithField("rollout", rollout.ID).WithError(err).Error("failed to create new configurations")
		}
		tx.Rollback(ctx)
		r.addErrorEvent(ctx, id, err)
		return
	}

	err = r.repo.RolloutEventCreate(ctx, &model.RolloutEvent{RolloutID: id, Type: model.RolloutEventTypeProcessed})
	if err != nil {
		log.WithError(err).Error("failed to create rollout event")
		r.addErrorEvent(ctx, id, err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.WithError(err).Error("failed to commit transaction")
		r.addErrorEvent(ctx, id, err)
	}
}

// getAndPrepare fetches the rollout from the database and prepares it for processing.
// The main result of this function is to update the rollout with the existing values for the feature.
func (r *Rollout) getAndPrepare(ctx context.Context, repo RolloutRepo, id uuid.UUID) (*model.Rollout, error) {
	rollout, err := repo.RolloutGetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if rollout.Status != model.RolloutStatusUnknown {
		return rollout, nil
	}

	_, env, err := r.tenantAndEnv(ctx, rollout.EnvironmentKind)
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

func (r *Rollout) tenantAndEnv(ctx context.Context, envKind model.EnvironmentKind) (*model.Tenant, *model.Environment, error) {
	var err error
	tenant, err := r.repo.TenantCI(ctx)
	if err != nil {
		return nil, nil, err
	}

	env, err := r.repo.EnvironmentCI(ctx, envKind)
	if err != nil {
		return nil, nil, err
	}
	return tenant, env, nil
}

func (r *Rollout) addEvent(ctx context.Context, rolloutID uuid.UUID, typ model.RolloutEventType, data json.RawMessage) {
	err := r.repo.RolloutEventCreate(ctx, &model.RolloutEvent{RolloutID: rolloutID, Type: typ, Data: data})
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"rollout": rolloutID,
			"type":    typ,
		}).WithError(err).Error("failed to create rollout event")
	}
}

func (r *Rollout) addErrorEvent(ctx context.Context, rolloutID uuid.UUID, err error) {
	e := strconv.Quote(err.Error())
	r.addEvent(ctx, rolloutID, model.RolloutEventTypeFailed, json.RawMessage(fmt.Sprintf(`{"error": "%v"}`, e)))
}
