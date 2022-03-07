package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/status"
	"github.com/sirupsen/logrus"
)

type Receiver struct {
	manager      *status.Manager[status.Update]
	subscriberID string
	repo         *database.Repo
	log          *logrus.Entry
}

func NewReceiver(mgr *status.Manager[status.Update], subscriberID string, repo *database.Repo, log *logrus.Entry) *Receiver {
	receiver := &Receiver{manager: mgr, subscriberID: subscriberID, repo: repo, log: log}
	return receiver
}

func (r *Receiver) Run(ctx context.Context) {
	err := r.manager.Receive(ctx, r.subscriberID, r.handler)
	if err != nil {
		r.log.WithError(err).Error("receive status messages")
	}
}

func (r *Receiver) handler(ctx context.Context, message status.Update) error {
	if message.Type != status.UpdateTypeHelm {
		return nil
	}
	helmStatus := &status.Helm{}
	err := json.Unmarshal(message.Data, helmStatus)
	if err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	environmentID, err := r.repo.EnvironmentIDByNames(ctx, message.Partner, message.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.WithField("partner", message.Partner).
				WithField("environment", message.Environment).
				Warn("unknown partner and/or environment")
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
