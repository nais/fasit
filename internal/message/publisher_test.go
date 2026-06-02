package message

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"cloud.google.com/go/pubsub/v2"
)

func TestPublisher(t *testing.T) {
	type testmsg struct {
		Name string
	}

	topic := &mocktopic{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	p := Publisher[testmsg]{
		topic: topic,
		log:   log,
	}

	err := p.Publish(context.Background(), testmsg{Name: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	p.Stop()

	if len(topic.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(topic.messages))
	}
	if !bytes.Equal(topic.messages[0].Data, []byte(`{"Name":"hello"}`)) {
		t.Errorf("Expected different message, got %v", string(topic.messages[0].Data))
	}
	if !topic.stopped {
		t.Error("Expected topic to be stopped")
	}
}

type mocktopic struct {
	messages []*pubsub.Message
	stopped  bool
}

func (m *mocktopic) Publish(ctx context.Context, msg *pubsub.Message) *pubsub.PublishResult {
	m.messages = append(m.messages, msg)
	return &pubsub.PublishResult{}
}

func (m *mocktopic) String() string { return "topic" }

func (m *mocktopic) Stop() {
	m.stopped = true
}
