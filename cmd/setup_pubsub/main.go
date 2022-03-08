package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
)

var (
	projectID          = "nais-io"
	statusSubscription = "fasit-subscription"
	statusTopic        = "naisd-status"
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.CreateTopic(ctx, statusTopic)
	if err != nil {
		log.Println(err)
	}

	_, err = client.CreateSubscription(ctx, statusSubscription, pubsub.SubscriptionConfig{
		Topic: client.Topic(statusTopic),
	})
	if err != nil {
		log.Println(err)
	}

	fmt.Println("ok")
}
