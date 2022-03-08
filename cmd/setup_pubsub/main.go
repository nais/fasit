package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
)

var (
	projectID          = "banankake"
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

	for partner, envs := range envs {
		for _, env := range envs {
			subscription := fmt.Sprintf("naisd-%v-%v-subscription", partner, env)
			topic := fmt.Sprintf("naisd-%v-%v", partner, env)

			_, err = client.CreateTopic(ctx, topic)
			if err != nil {
				log.Println(err)
			}

			_, err = client.CreateSubscription(ctx, subscription, pubsub.SubscriptionConfig{
				Topic: client.Topic(topic),
			})
			if err != nil {
				log.Println(err)
			}
		}
	}

	fmt.Println("ok")
}
