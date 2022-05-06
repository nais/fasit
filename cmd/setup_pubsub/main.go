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
		"test-partner": {"dev", "management"},
	}
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
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

	for tenant, envs := range envs {
		for _, env := range envs {
			topic := fmt.Sprintf("naisd-%v-%v", tenant, env)
			subscription := "naisd-subscription"

			_, err = client.CreateTopic(ctx, topic)
			if err != nil {
				log.Println(err)
			}

			envClient, err := pubsub.NewClient(ctx, "local-"+tenant+"-"+env)
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

	topics := client.Topics(ctx)
	fmt.Println("TOPICS:")
	for {
		ts, err := topics.Next()
		if err != nil {
			log.Println(err)
			break
		}

		log.Println(ts.String())
	}

	subs := client.Subscriptions(ctx)
	fmt.Println("SUBSCRIPTIONS:")
	for {
		ts, err := subs.Next()
		if err != nil {
			log.Println(err)
			break
		}

		log.Println(ts.String())
	}

	fmt.Println("ok")
}
