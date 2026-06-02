package message

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	topic           Topic
	log             *slog.Logger
	config          publisherConfig
	publishCount    metric.Int64Counter
	publishDuration metric.Float64Histogram
	topicName       string
}

func NewPublisher[T any](client *pubsub.Client, projectID, topicID string, log *slog.Logger, opts ...publisherOpts) *Publisher[T] {
	cfg := publisherConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Publisher[T]{
		topic:     client.Publisher("projects/" + projectID + "/topics/" + topicID),
		log:       log,
		config:    cfg,
		topicName: topicID,
	}
}

func (p *Publisher[T]) SetMeter(meter metric.Meter) {
	var err error
	p.publishCount, err = meter.Int64Counter("pubsub_messages_published_total",
		metric.WithDescription("Total pub/sub messages published"),
	)
	if err != nil {
		p.log.Warn("failed to create publish counter", "error", err)
	}

	p.publishDuration, err = meter.Float64Histogram("pubsub_publish_duration_seconds",
		metric.WithDescription("Pub/sub publish duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		p.log.Warn("failed to create publish duration histogram", "error", err)
	}
}

func (p *Publisher[T]) Publish(ctx context.Context, msg T) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	start := time.Now()

	p.log.Debug("Published message", "topic", p.topic.String())
	res := p.topic.Publish(ctx, &pubsub.Message{
		Data:       data,
		Attributes: p.config.attributes,
	})

	if p.config.waitForPublish {
		if _, err := res.Get(ctx); err != nil {
			p.recordPublish(ctx, start, true)
			return err
		}
	}

	p.recordPublish(ctx, start, false)
	return nil
}

func (p *Publisher[T]) recordPublish(ctx context.Context, start time.Time, isErr bool) {
	attrs := metric.WithAttributes(
		attribute.String("topic", p.topicName),
		attribute.Bool("error", isErr),
	)
	if p.publishCount != nil {
		p.publishCount.Add(ctx, 1, attrs)
	}
	if p.publishDuration != nil {
		p.publishDuration.Record(ctx, time.Since(start).Seconds(), attrs)
	}
}

func (p *Publisher[T]) Stop() {
	p.topic.Stop()
}
