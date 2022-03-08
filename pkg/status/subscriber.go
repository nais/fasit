package status

import (
	"context"
	"encoding/json"
	"log"

	"cloud.google.com/go/pubsub"
)

type Subscriber[T any] struct {
	subscription *pubsub.Subscription
}

func NewSubscriber[T any](client *pubsub.Client, subscriptionID string) (*Subscriber[T], error) {
	return &Subscriber[T]{
		subscription: client.Subscription(subscriptionID),
	}, nil
}

func (s *Subscriber[T]) Receive(ctx context.Context, f func(ctx context.Context, msg T) error) error {
	return s.subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		var t T
		if err := json.Unmarshal(msg.Data, &t); err != nil {
			log.Println(err)
			msg.Ack()
			return
		}

		if err := f(ctx, t); err != nil {
			log.Println(err)
			msg.Nack()
			return
		}

		msg.Ack()
	})
}
