package database

import (
	"context"
	"errors"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
)

type ListenFunc func(context.Context, uuid.UUID)

func (r *repo) ListenNotify(ctx context.Context, channel string, fn ListenFunc) error {
	log := r.log.WithField("query", channel)

	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	log.Info("listener started")

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			switch {
			case pgconn.Timeout(err):
				return nil
			case errors.Is(err, io.ErrUnexpectedEOF):
				log.Infof("listener got unexpected EOF, retry")
				continue
			}

			log.WithError(err).Errorf("error waiting for notification: %T", err)
			return nil
		}

		id, err := uuid.Parse(notification.Payload)
		if err != nil {
			log.WithError(err).Error("error parsing notification payload")
			return nil
		}

		fn(ctx, id)
	}
}
