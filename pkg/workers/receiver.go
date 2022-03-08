package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type Receiver struct {
	manager *message.Subscriber[message.Status]
	repo    *database.Repo
	log     *logrus.Entry
}

func NewReceiver(mgr *message.Subscriber[message.Status], repo *database.Repo, log *logrus.Entry) *Receiver {
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

	environmentID, err := r.repo.EnvironmentIDByNames(ctx, msg.Partner, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.WithField("partner", msg.Partner).
				WithField("environment", msg.Environment).
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
