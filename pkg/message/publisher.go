package message

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
)

type Publisher[T any] struct {
	topic *pubsub.Topic
}

func NewPublisher[T any](client *pubsub.Client, projectID, topicID string) *Publisher[T] {
	return &Publisher[T]{
		topic: client.TopicInProject(topicID, projectID),
	}
}

func (m *Publisher[T]) Publish(ctx context.Context, msg T) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	fmt.Println("Publish to", m.topic.String(), msg)
	m.topic.Publish(ctx, &pubsub.Message{
		Data: data,
	})

	return nil
}

func (m *Publisher[T]) Stop() {
	m.topic.Stop()
}
