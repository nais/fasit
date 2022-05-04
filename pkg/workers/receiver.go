package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type ReceiverStore interface {
	EnvironmentIDByNames(ctx context.Context, tenantName string, environmentName string) (uuid.UUID, error)
	StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Helm) error
	ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error
}

type Receiver struct {
	manager ReceiverClient
	repo    ReceiverStore
	log     *logrus.Entry
}

func NewReceiver(mgr ReceiverClient, repo ReceiverStore, log *logrus.Entry) *Receiver {
	receiver := &Receiver{manager: mgr, repo: repo, log: log}
	return receiver
}

func (r *Receiver) Run(ctx context.Context) {
	err := r.manager.Receive(ctx, r.handler)
	if err != nil {
		r.log.WithError(err).Error("receive status messages")
	}
}

func (r *Receiver) handler(ctx context.Context, msg message.Status) error {
	switch msg.Type {
	case message.StatusTypeHelm:
		return r.handlerHelm(ctx, msg)
	}

	return nil
}

func (r *Receiver) handlerHelm(ctx context.Context, msg message.Status) error {
	helmStatus := &message.Helm{}
	err := json.Unmarshal(msg.Data, helmStatus)
	if err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	environmentID, err := r.repo.EnvironmentIDByNames(ctx, msg.Tenant, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.WithField("tenant", msg.Tenant).
				WithField("environment", msg.Environment).
				Warn("unknown tenant and/or environment")
			return nil
		}
		return err
	}
	err = r.repo.StatusCreateOrUpdate(ctx, environmentID, helmStatus)
	if err != nil {
		return err
	}

	return nil
}

func (r *Receiver) releaseStatus(ctx context.Context, msg message.Status) error {
	status := &message.HelmRelease{}
	err := json.Unmarshal(msg.Data, status)
	if err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	environmentID, err := r.repo.EnvironmentIDByNames(ctx, msg.Tenant, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.WithField("tenant", msg.Tenant).
				WithField("environment", msg.Environment).
				Warn("unknown tenant and/or environment")
			return nil
		}
		return err
	}

	for _, rel := range status.Releases {
		err = r.repo.ReleaseStatusCreateOrUpdate(ctx, environmentID, &rel)
		if err != nil {
			return err
		}
	}

	return nil
}
