package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/joho/godotenv"
)

var (
	naisProjectID      = "nais-local-dev"
	statusSubscription = "fasit-subscription"
	statusTopic        = "status"
)

func main() {
	_ = godotenv.Load()

	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, naisProjectID)
	if err != nil {
		log.Fatalf("%v", err)
	}

	tenantEnvs := os.Getenv("PUBSUB_TENANT_ENVS")
	parts := strings.Split(tenantEnvs, ",")

	topicRes, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/" + naisProjectID + "/topics/" + statusTopic,
	})
	if err != nil {
		println("err:", err)
	}
	_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:  "projects/" + naisProjectID + "/subscriptions/" + statusSubscription,
		Topic: topicRes.GetName(),
	})
	if err != nil {
		println("err:", err)
	}

	for _, tenantEnv := range parts {
		p := strings.Split(tenantEnv, ":")
		tenant := p[0]
		env := p[1]
		topic := fmt.Sprintf("projects/%v/topics/naisd-%v-%v", naisProjectID, tenant, env)
		subscription := fmt.Sprintf("projects/%v/subscriptions/naisd-subscription", "local-"+tenant+"-"+env)

		_, err = client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topic})
		if err != nil {
			log.Println(err)
		}

		envClient, err := pubsub.NewClient(ctx, "local-"+tenant+"-"+env)
		if err != nil {
			log.Fatal(err)
		}
		_, err = envClient.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
			Name:  subscription,
			Topic: topic,
		})
		if err != nil {
			log.Println(err)
		}
		log.Printf("Created topic %v\n", topic)
	}
}
