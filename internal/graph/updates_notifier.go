package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
)

type updateNotifier struct {
	repo database.Repo

	lock        sync.RWMutex
	subscribers map[chan<- model.Update]struct{}
}

func newDeployInstructionsNotifier(ctx context.Context, not *notifier.Notifier, repo database.Repo) *updateNotifier {
	chDI := not.Listen("deploy_instructions")
	chCfgGlobal := not.Listen("configurations_global")
	chCfgEnv := not.Listen("configurations_environment")
	states := not.Listen("feature_states")
	clusterUpgrades := not.Listen("cluster_upgrades")

	lf := &updateNotifier{
		repo:        repo,
		subscribers: make(map[chan<- model.Update]struct{}),
	}

	go func() {
		var _ <-chan notifier.Payload = clusterUpgrades
		lf.run(ctx, chDI, chCfgGlobal, chCfgEnv, states)
	}()

	return lf
}

func (d *updateNotifier) Subscribe(ch chan<- model.Update) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.subscribers[ch] = struct{}{}
}

func (d *updateNotifier) Unsubscribe(ch chan<- model.Update) {
	d.lock.Lock()
	defer d.lock.Unlock()

	delete(d.subscribers, ch)
}

func (d *updateNotifier) run(ctx context.Context, di, global, env, states <-chan notifier.Payload) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-di:
			d.handleDeployInstruction(ctx, msg)
		case msg := <-global:
			d.handleConfig(ctx, msg)
		case msg := <-env:
			d.handleConfig(ctx, msg)
		case msg := <-states:
			d.handleFeatureState(ctx, msg)
		}
	}
}

func (d *updateNotifier) handleDeployInstruction(ctx context.Context, msg notifier.Payload) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	id, err := d.getAsUUID("id", msg)
	if err != nil {
		logrus.Debug("failed to get deploy instruction id")
		return
	}

	di, err := d.repo.DeployInstructionGet(ctx, id)
	if err != nil {
		logrus.Debug("failed to get deploy instruction")
		return
	}

	for sub := range d.subscribers {
		select {
		case sub <- &model.Status{
			Status:              di.Status,
			EnvironmentID:       di.EnvironmentID,
			Feature:             di.FeatureName,
			Version:             di.FeatureVersion,
			ConfigHash:          di.Hash,
			Created:             di.Created,
			LastModified:        di.LastModified,
			DeployInstructionID: di.ID,
		}:
		default:
			logrus.Debug("subscriber blocked")
		}
	}
}

func (d *updateNotifier) handleConfig(ctx context.Context, msg notifier.Payload) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	id, err := d.getAsUUID("id", msg)
	if err != nil {
		logrus.Debug("failed to get config id")
		return
	}

	cfg, err := featurepkg.ConfigGetByID(ctx, id)
	if err != nil {
		logrus.Debug("failed to get global config")
		return
	}

	if msg.Table == "configurations_environment" {
		cfg.Source = model.ConfigSourceEnv
	}

	for sub := range d.subscribers {
		select {
		case sub <- cfg:
		default:
			logrus.Debug("subscriber blocked")
		}
	}
}

func (d *updateNotifier) handleFeatureState(ctx context.Context, msg notifier.Payload) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	envid, err := d.getAsUUID("environment_id", msg)
	if err != nil {
		logrus.Debug("failed to get feature state id")
		return
	}

	feature, err := d.getAsString("feature", msg)
	if err != nil {
		logrus.Debug("failed to get feature state feature")
		return
	}

	fs, err := featurepkg.FeatureStateGet(ctx, envid, feature)
	if err != nil {
		logrus.Debug("failed to get feature state")
		return
	}

	for sub := range d.subscribers {
		select {
		case sub <- fs:
		default:
			logrus.Debug("subscriber blocked")
		}
	}
}

func (d *updateNotifier) getAsUUID(field string, msg notifier.Payload) (uuid.UUID, error) {
	str, err := d.getAsString(field, msg)
	if err != nil {
		return uuid.Nil, err
	}

	return uuid.Parse(str)
}

func (d *updateNotifier) getAsString(field string, msg notifier.Payload) (string, error) {
	v, ok := msg.Data[field]
	if !ok || v == nil {
		return "", fmt.Errorf("missing id in message")
	}
	str, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("id is not a number")
	}

	return str, nil
}
