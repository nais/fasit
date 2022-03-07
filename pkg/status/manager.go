package status

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud.google.com/go/pubsub"
)

type Manager[T any] struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

func New[T any](ctx context.Context, projectID, topicID string) (*Manager[T], error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	mgr := &Manager[T]{
		client: client,
	}

	if err := mgr.ensureTopic(ctx, topicID); err != nil {
		return nil, err
	}

	return mgr, nil
}

func (m *Manager[T]) ensureTopic(ctx context.Context, topicID string) error {
	m.topic = m.client.Topic(topicID)
	exists, err := m.topic.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		fmt.Println("exists", topicID)
		return nil
	}

	m.topic, err = m.client.CreateTopic(ctx, topicID)
	if err != nil {
		return err
	}
	fmt.Println("created", topicID)

	return nil
}

func (m *Manager[T]) Publish(ctx context.Context, msg T) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	m.topic.Publish(ctx, &pubsub.Message{
		Data: data,
	})
	return nil
}

func (m *Manager[T]) Receive(ctx context.Context, subID string, fn func(context.Context, T) error) error {
	sub, err := m.ensureSubscription(ctx, subID)
	if err != nil {
		return err
	}
	return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		fmt.Println(string(msg.Data))
		var su T
		if err := json.Unmarshal(msg.Data, &su); err != nil {
			log.Println(err)
			msg.Nack()
			return
		}
		if err := fn(ctx, su); err != nil {
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

func (m *Manager[T]) ensureSubscription(ctx context.Context, subID string) (*pubsub.Subscription, error) {
	sub := m.client.Subscription(subID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return sub, nil
	}

	return m.client.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
		Topic: m.topic,
	})
}

func (m *Manager[T]) SubscribeTopic(ctx context.Context, name, topicID string) (*pubsub.Subscription, error) {
	return m.client.CreateSubscription(ctx, name, pubsub.SubscriptionConfig{
		Topic: m.client.Topic(topicID),
	})
}

func (m *Manager[T]) StopTopic() {
	m.topic.Stop()
}

func (m *Manager[T]) Close() error {
	return m.client.Close()
}
