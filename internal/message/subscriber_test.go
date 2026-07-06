package message

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"
	"github.com/nais/fasit/internal/ioconvenience"
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
	defer ioconvenience.CloseWithLog(srv, slog.Default())
	// Connect to the server without using TLS.
	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer ioconvenience.CloseWithLog(conn, slog.Default())
	// Use the connection when creating a pubsub client.
	client, err := pubsub.NewClient(ctx, "project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	defer ioconvenience.CloseWithLog(client, slog.Default())

	topicRes, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/project/topics/topic",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:  "projects/project/subscriptions/subscription",
		Topic: topicRes.GetName(),
	})
	if err != nil {
		t.Fatal(err)
	}

	topic := client.Publisher(topicRes.GetName())
	topic.Publish(ctx, &pubsub.Message{Data: []byte(`{"Name":"test"}`)})

	type testmsg struct{ Name string }
	log := slog.Default()
	// log.SetOutput(io.Discard)
	sub := NewSubscriber[testmsg](client, "project", "subscription", log)
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
