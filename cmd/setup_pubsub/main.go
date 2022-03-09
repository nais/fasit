package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
)

var (
	naisProjectID      = "nais-local-dev"
	statusSubscription = "fasit-subscription"
	statusTopic        = "status"

	envs = map[string][]string{
		"test": {"dev"},
	}
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, naisProjectID)
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

	for partner, envs := range envs {
		for _, env := range envs {
			topic := fmt.Sprintf("naisd-%v-%v", partner, env)
			subscription := "naisd-subscription"

			_, err = client.CreateTopic(ctx, topic)
			if err != nil {
				log.Println(err)
			}

			envClient, err := pubsub.NewClient(ctx, "local-"+partner+"-"+env)
			if err != nil {
				log.Fatal(err)
			}
			_, err = envClient.CreateSubscription(ctx, subscription, pubsub.SubscriptionConfig{
				Topic: envClient.TopicInProject(topic, naisProjectID),
			})
			if err != nil {
				log.Println(err)
			}
		}
	}

	fmt.Println("ok")
}
