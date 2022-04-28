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
	if msg.Type != message.StatusTypeHelm {
		return nil
	}
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
