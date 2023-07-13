package graph

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/notifier"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

type updateNotifier struct {
	repo database.Repo
	msgs chan *notifier.Payload

	lock        sync.RWMutex
	subscribers map[chan<- model.Update]struct{}
}

func newDeployInstructionsNotifier(ctx context.Context, not *notifier.Notifier, repo database.Repo) *updateNotifier {
	chDI := not.Listen("deploy_instructions")
	chCfgGlobal := not.Listen("configurations_global")
	chCfgEnv := not.Listen("configurations_environment")

	lf := &updateNotifier{
		repo:        repo,
		subscribers: make(map[chan<- model.Update]struct{}),
	}

	go lf.run(ctx, chDI, chCfgGlobal, chCfgEnv)

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

func (d *updateNotifier) run(ctx context.Context, ch, global, env <-chan notifier.Payload) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			d.handleMessage(ctx, msg)
		case msg := <-global:
			d.handleMessage(ctx, msg)
		case msg := <-env:
			d.handleMessage(ctx, msg)
		}
	}
}

func (d *updateNotifier) handleMessage(ctx context.Context, msg notifier.Payload) {
	lid, ok := msg.Data["id"]
	if !ok || lid == nil {
		logrus.Debug("missing id in message")
		return
	}
	lidstr, ok := lid.(string)
	if !ok {
		logrus.Debug("id is not a number")
		return
	}

	diid, err := uuid.Parse(lidstr)
	if err != nil {
		logrus.Debug("invalid id")
		return
	}

	switch msg.Table {
	case "deploy_instructions":
		d.handleDeployInstruction(ctx, diid)
	case "configurations_global", "configurations_environment":
		d.handleConfig(ctx, diid, msg.Table == "configurations_environment")
	}
}

func (d *updateNotifier) handleDeployInstruction(ctx context.Context, id uuid.UUID) {
	d.lock.RLock()
	defer d.lock.RUnlock()
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

func (d *updateNotifier) handleConfig(ctx context.Context, id uuid.UUID, env bool) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	cfg, err := d.repo.ConfigGetByID(ctx, id)
	if err != nil {
		logrus.Debug("failed to get global config")
		return
	}

	if env {
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
