package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nais/fasit/pkg/integration_test/testmanager/runner"
)

type pubsubMockMsg struct {
	topic string
	msg   []byte
}

type mockSubscriber[T any] struct {
	envName    string
	tenantName string
	messages   chan pubsubMockMsg
	done       <-chan struct{}
	pubsub     *runner.PubSub
}

func (d *mockSubscriber[T]) Name() string {
	return "naisd-" + d.tenantName + "-" + d.envName
}

func (d *mockSubscriber[T]) Synchronous() {}

func (d *mockSubscriber[T]) Receive(ctx context.Context, f func(ctx context.Context, msg T) error) error {
	for {
		fmt.Println("############## Receive message")
		select {
		case <-d.done:
			fmt.Println("######## DONE asdf")
			return nil
		case <-ctx.Done():
			fmt.Println("###### CTX DONe")
			return nil
		case msg := <-d.messages:
			fmt.Println("########## GOT MESSAGE")
			var m T
			mp := make(map[string]any)
			if err := json.Unmarshal(msg.msg, &m); err != nil {
				return err
			}
			if err := json.Unmarshal(msg.msg, &mp); err != nil {
				return err
			}
			d.pubsub.Receive(msg.topic, runner.PubSubMessage{Msg: mp})

			fmt.Println("Find message")
			if err := f(ctx, m); err != nil {
				return err
				log.Println(err)
			}
			fmt.Println("Done find")
		}
		fmt.Println("######## Rerun loop")
	}
}

type mockPublisher[T any] struct {
	topic    string
	messages chan<- pubsubMockMsg
	pubsub   *runner.PubSub
}

func (m *mockPublisher[T]) Publish(ctx context.Context, msg T) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	m.messages <- pubsubMockMsg{
		topic: m.topic,
		msg:   b,
	}

	mp := make(map[string]any)
	if err := json.Unmarshal(b, &mp); err != nil {
		return err
	}
	m.pubsub.Send(m.topic, runner.PubSubMessage{Msg: mp})

	return nil
}

func (m *mockPublisher[T]) Stop() {}
