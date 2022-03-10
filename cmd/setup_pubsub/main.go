package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"cloud.google.com/go/pubsub"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
)

var (
	naisProjectID      = "nais-local-dev"
	statusSubscription = "fasit-subscription"
	statusTopic        = "status"

	envs = map[string][]string{
		"test": {"dev"},
	}
	databaseName = "projects/nais-local-dev/instances/fasit/databases/fasit"
)

func init() {
	flag.StringVar(&databaseName, "database", databaseName, "A valid database name has the form projects/PROJECT_ID/instances/INSTANCE_ID/databases/DATABASE_ID")
}

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		log.Fatal(err)
	}

	if err := os.Setenv("SPANNER_EMULATOR_HOST", "localhost:9010"); err != nil {
		log.Fatal(err)
	}

	flag.Parse()

	ctx := context.Background()
	if err := createDatabase(ctx, databaseName); err != nil {
		log.Fatal(err)
	}

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

func createDatabase(ctx context.Context, db string) error {
	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		return fmt.Errorf("invalid database id %s", db)
	}

	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer adminClient.Close()

	op, err := adminClient.CreateDatabase(ctx, &adminpb.CreateDatabaseRequest{
		Parent:          matches[1],
		CreateStatement: "CREATE DATABASE `" + matches[2] + "`",
	})
	if err != nil {
		return err
	}
	if _, err := op.Wait(ctx); err != nil {
		return err
	}
	fmt.Printf("Created database [%s]\n", db)
	return nil
}
