package naisd

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type pubsubLogger struct {
	diid  uuid.UUID
	topic StatusPublisher
	lock  sync.Mutex
	lines []message.LogLine

	log logrus.FieldLogger
}

func newPubsubLogger(diid uuid.UUID, topic StatusPublisher, log logrus.FieldLogger) *pubsubLogger {
	return &pubsubLogger{
		diid:  diid,
		topic: topic,
		log:   log,
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
