package naisd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type pubsubLogger struct {
	diid  uuid.UUID
	topic StatusPublisher
	lock  sync.Mutex
	lines []message.LogLine

	log   logrus.FieldLogger
	close chan struct{}
}

func newPubsubLogger(diid uuid.UUID, topic StatusPublisher, log logrus.FieldLogger) *pubsubLogger {
	return &pubsubLogger{
		diid:  diid,
		topic: topic,
		log:   log,
		close: make(chan struct{}),
	}
}

func (p *pubsubLogger) Write(b []byte) (n int, err error) {
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))

	p.lock.Lock()
	defer p.lock.Unlock()

	for _, line := range lines {
		p.lines = append(p.lines, message.LogLine{
			Time: time.Now(),
			Msg:  string(line),
		})
	}

	return len(b), nil
}

func (p *pubsubLogger) AddEvent(event *corev1.Event) {
	b := strings.Builder{}
	b.WriteString(event.InvolvedObject.Name)
	b.WriteString(" (")
	b.WriteString(event.InvolvedObject.APIVersion)
	b.WriteString(event.InvolvedObject.Kind)
	b.WriteString("): ")
	b.WriteString(event.Message)

	p.lock.Lock()
	defer p.lock.Unlock()

	p.lines = append(p.lines, message.LogLine{
		Time: time.Now(),
		Msg:  b.String(),
		Kind: "event",
	})
}

func (p *pubsubLogger) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.Publish(context.Background()); err != nil {
				p.log.WithError(err).Error("publishing logs")
			}
		case <-ctx.Done():
			return
		case <-p.close:
			if err := p.Publish(context.Background()); err != nil {
				p.log.WithError(err).Error("publishing logs")
			}
			return
		}
	}
}

func (p *pubsubLogger) Publish(ctx context.Context) error {
	p.lock.Lock()
	lines := p.lines
	p.lines = nil
	p.lock.Unlock()

	if len(lines) == 0 {
		return nil
	}

	data := message.StatusLog{
		DIID: p.diid,
		Logs: lines,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return p.topic.Publish(ctx, message.Status{
		Type: message.StatusTypeLog,
		Data: b,
	})
}

func (p *pubsubLogger) Close() error {
	close(p.close)
	return p.Publish(context.Background())
}
