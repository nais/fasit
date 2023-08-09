package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/integration_test/testmanager/parser"
)

type PubSubTopic struct {
	Sent     []PubSubMessage
	Received []PubSubMessage
}

type PubSubMessage struct {
	Msg        map[string]any    `json:"msg"`
	Attributes map[string]string `json:"attributes"`
}

type PubSubHook func(topic string, msg PubSubMessage) error

type PubSub struct {
	lock      sync.Mutex
	topics    map[string]PubSubTopic
	doPublish PubSubHook
}

func NewPubSub(doPublish PubSubHook) *PubSub {
	return &PubSub{
		doPublish: doPublish,
	}
}

func (p *PubSub) Ext() string {
	return "pubsub"
}

func (p *PubSub) Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error {
	f, err := parser.Parse(body, state)
	if err != nil {
		return fmt.Errorf("gql.Parse: %w", err)
	}

	topic, ok := f.Opts["topic"]
	if !ok {
		return fmt.Errorf("missing 'topic' option")
	}
	if !p.hasTopic(topic) {
		return fmt.Errorf("topic %q not registered, has: %v", topic, p.topicsNames())
	}
	delete(f.Opts, "topic")

	if len(f.Query) > 0 && len(f.Returns) > 0 {
		return fmt.Errorf("cannot both query and return")
	}

	if len(f.Query) > 0 {
		psm := PubSubMessage{}
		if err := json.Unmarshal([]byte(f.Query), &psm); err != nil {
			return err
		}

		return p.doPublish(topic, psm)
	}

	// When RETURNS is defined
	msgs := p.messages(topic)
	if len(msgs) == 0 {
		return fmt.Errorf("no messages received on topic %q", topic)
	}

	cmpOpts, err := f.CmpOpts()
	if err != nil {
		return err
	}

	expected := map[string]any{}
	if err := json.Unmarshal([]byte(f.Returns), &expected); err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	var errs []error
	for _, msg := range msgs {
		if !cmp.Equal(msg.Msg, expected, cmpOpts...) {
			errs = append(errs, fmt.Errorf("diff -want +got:\n%v", cmp.Diff(expected, msg.Msg, cmpOpts...)))
		} else {
			// Ok
			f.AppendStore(msg.Msg, state)
			return nil
		}
	}

	for _, err := range errs {
		logf("%v", err)
	}

	return fmt.Errorf("no matching messages received on topic %q", topic)
}

func (p *PubSub) Send(topic string, msg PubSubMessage) {
	p.lock.Lock()
	defer p.lock.Unlock()

	t, ok := p.topics[topic]
	if !ok {
		t = PubSubTopic{}
	}

	t.Sent = append(t.Sent, msg)

	if p.topics == nil {
		p.topics = map[string]PubSubTopic{}
	}
	p.topics[topic] = t
}

func (p *PubSub) Receive(topic string, msg PubSubMessage) {
	p.lock.Lock()
	defer p.lock.Unlock()

	t, ok := p.topics[topic]
	if !ok {
		t = PubSubTopic{}
	}

	t.Received = append(t.Received, msg)
	if p.topics == nil {
		p.topics = map[string]PubSubTopic{}
	}
	p.topics[topic] = t
}

func (p *PubSub) hasTopic(name string) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	_, ok := p.topics[name]
	return ok
}

func (p *PubSub) topicsNames() []string {
	p.lock.Lock()
	defer p.lock.Unlock()

	names := []string{}
	for k := range p.topics {
		names = append(names, k)
	}

	return names
}

func (p *PubSub) messages(topic string) []PubSubMessage {
	p.lock.Lock()
	defer p.lock.Unlock()

	t, ok := p.topics[topic]
	if !ok {
		return nil
	}

	return t.Received
}
