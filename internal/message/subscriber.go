package message

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"
)

type contextKey int

var ErrNack = errors.New("nack")

const ackContext = contextKey(1)

func ForceAck(ctx context.Context) {
	msg := ctx.Value(ackContext)
	if msg == nil {
		return
	}
	msg.(*pubsub.Message).Ack()
}

type Subscriber[T any] struct {
	subscription *pubsub.Subscriber
	log          *slog.Logger
}

func NewSubscriber[T any](client *pubsub.Client, projectID, subscriptionID string, log *slog.Logger) *Subscriber[T] {
	return &Subscriber[T]{
		subscription: client.Subscriber("projects/" + projectID + "/subscriptions/" + subscriptionID),
		log:          log.With("subsystem", "status-subscriber"),
	}
}

func (s *Subscriber[T]) Name() string {
	return s.subscription.String()
}

func (s *Subscriber[T]) Synchronous() {
	s.subscription.ReceiveSettings.MaxOutstandingMessages = 1
}

func (s *Subscriber[T]) Receive(ctx context.Context, f func(ctx context.Context, msg T) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err := s.receive(ctx, f); err != nil {
			s.log.With("err", err).Error("subscriber error during receive")
		}
	}
}

func (s *Subscriber[T]) receive(ctx context.Context, f func(ctx context.Context, msg T) error) error {
	return s.subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		ctx = context.WithValue(ctx, ackContext, msg)
		var t T
		if err := json.Unmarshal(msg.Data, &t); err != nil {
			s.log.With("err", err).Error("unmarshal message")
			msg.Ack()
			return
		}

		if err := f(ctx, t); err != nil {
			if !errors.Is(err, ErrNack) {
				s.log.With("err", err).Error("handle message")
			}
			msg.Nack()
			return
		}

		msg.Ack()
	})
}
