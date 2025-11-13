package message

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub/v2"
	"github.com/sirupsen/logrus"
)

type publisherConfig struct {
	waitForPublish bool
	attributes     map[string]string
}

type publisherOpts func(*publisherConfig)

func WithWaithForPublish() publisherOpts {
	return func(c *publisherConfig) {
		c.waitForPublish = true
	}
}

func WithAttributes(attrs map[string]string) publisherOpts {
	return func(c *publisherConfig) {
		c.attributes = attrs
	}
}

type Topic interface {
	Publish(ctx context.Context, msg *pubsub.Message) *pubsub.PublishResult
	String() string
	Stop()
}

type Publisher[T any] struct {
	topic  Topic
	log    logrus.FieldLogger
	config publisherConfig
}

func NewPublisher[T any](client *pubsub.Client, projectID, topicID string, log logrus.FieldLogger, opts ...publisherOpts) *Publisher[T] {
	cfg := publisherConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Publisher[T]{
		topic:  client.Publisher("projects/" + projectID + "/topics/" + topicID),
		log:    log,
		config: cfg,
	}
}

func (p *Publisher[T]) Publish(ctx context.Context, msg T) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	p.log.WithField("topic", p.topic.String()).Debug("Published message")
	res := p.topic.Publish(ctx, &pubsub.Message{
		Data:       data,
		Attributes: p.config.attributes,
	})

	if p.config.waitForPublish {
		if _, err := res.Get(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (p *Publisher[T]) Stop() {
	p.topic.Stop()
}
