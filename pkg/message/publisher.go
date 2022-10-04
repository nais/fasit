package message

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub"
	"github.com/sirupsen/logrus"
)

type Topic interface {
	Publish(ctx context.Context, msg *pubsub.Message) *pubsub.PublishResult
	String() string
	Stop()
}

type Publisher[T any] struct {
	topic Topic
	log   *logrus.Entry
}

func NewPublisher[T any](client *pubsub.Client, projectID, topicID string, log *logrus.Entry) *Publisher[T] {
	return &Publisher[T]{
		topic: client.TopicInProject(topicID, projectID),
		log:   log,
	}
}

func (p *Publisher[T]) Publish(ctx context.Context, msg T) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	p.log.WithField("topic", p.topic.String()).Debug("Published message")
	res := p.topic.Publish(ctx, &pubsub.Message{
		Data: data,
	})

	if _, err := res.Get(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Publisher[T]) Stop() {
	p.topic.Stop()
}
