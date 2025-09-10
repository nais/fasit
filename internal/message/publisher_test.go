package message

import (
	"bytes"
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/sirupsen/logrus"
)

func TestPublisher(t *testing.T) {
	type testmsg struct {
		Name string
	}

	topic := &mocktopic{}
	buf := &bytes.Buffer{}
	log := logrus.New()
	log.Out = buf
	log.Level = logrus.DebugLevel

	p := Publisher[testmsg]{
		topic: topic,
		log:   log.WithTime(time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)),
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
	if buf.String() != "time=\"2020-01-01T00:00:00Z\" level=debug msg=\"Published message\" topic=topic\n" {
		t.Errorf("Expected different log, got %q", buf.String())
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
