package message

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestSubscriber(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	// Connect to the server without using TLS.
	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Use the connection when creating a pubsub client.
	client, err := pubsub.NewClient(ctx, "project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	topic, err := client.CreateTopic(ctx, "topic")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateSubscription(ctx, "subscription", pubsub.SubscriptionConfig{Topic: client.Topic("topic")})
	if err != nil {
		t.Fatal(err)
	}
	topic.Publish(ctx, &pubsub.Message{Data: []byte(`{"Name":"test"}`)})

	type testmsg struct{ Name string }
	sub := NewSubscriber[testmsg](client, "project", "subscription")
	sub.Synchronous()

	ctx, done := context.WithCancel(ctx)
	msgs := []testmsg{}
	err = sub.Receive(ctx, func(ctx context.Context, msg testmsg) error {
		msgs = append(msgs, msg)
		done()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	<-ctx.Done()

	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}
