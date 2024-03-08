package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/nais/fasit/pkg/provider/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	naisProjectID      = "nais-local-dev"
	statusSubscription = "fasit-subscription"
	statusTopic        = "status"

	envs = map[string]map[string]protogen.EnvironmentKind{
		"test-partner": {"dev": protogen.EnvironmentKind_TENANT, "management": protogen.EnvironmentKind_MANAGEMENT},
		"nav": {
			"management": protogen.EnvironmentKind_MANAGEMENT,
			"dev":        protogen.EnvironmentKind_TENANT,
			"prod":       protogen.EnvironmentKind_TENANT,
			"dev-fss":    protogen.EnvironmentKind_ONPREM,
			"prod-fss":   protogen.EnvironmentKind_ONPREM,
			"dev-gcp":    protogen.EnvironmentKind_LEGACY,
			"prod-gcp":   protogen.EnvironmentKind_LEGACY,
		},
		"ssb":      {"dev": protogen.EnvironmentKind_TENANT, "management": protogen.EnvironmentKind_MANAGEMENT},
		"fhi":      {"dev": protogen.EnvironmentKind_TENANT, "management": protogen.EnvironmentKind_MANAGEMENT},
		"dev-nais": {"dev": protogen.EnvironmentKind_TENANT, "management": protogen.EnvironmentKind_MANAGEMENT},
	}
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	conn, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	grpcClient := protogen.NewProviderClient(conn)
	for tenantName, environments := range envs {
		_, err := grpcClient.CreateTenant(ctx, &protogen.CreateTenantRequest{Name: tenantName})
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				log.Fatal(err)
			}
		}
		tenant, err := grpcClient.GetTenant(ctx, &protogen.GetTenantRequest{Name: tenantName})
		if err != nil {
			log.Fatal(err)
		}
		for env, kind := range environments {
			_, err := grpcClient.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
				TenantId: tenant.Id,
				Name:     env,
				Kind:     kind,
			})
			if err != nil {
				if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					log.Fatal(err)
				}
			}

			environment, err := grpcClient.GetEnvironment(ctx, &protogen.GetEnvironmentRequest{TenantId: tenant.Id, Name: env})
			if err != nil {
				log.Fatal(err)
			}

			_, err = grpcClient.CreateOrUpdateEnvironmentValue(ctx, &protogen.CreateOrUpdateEnvironmentValueRequest{
				EnvironmentId: environment.Id,
				Key:           "project_id",
				Value:         json.RawMessage(fmt.Sprintf("%q", "nais-"+env)),
			})
			if err != nil {
				log.Fatal(err)
			}
			_, err = grpcClient.CreateOrUpdateEnvironmentValue(ctx, &protogen.CreateOrUpdateEnvironmentValueRequest{
				EnvironmentId: environment.Id,
				Key:           "updated_at",
				Value:         json.RawMessage(fmt.Sprintf("%q", time.Now().String())),
			})
			if err != nil {
				log.Fatal(err)
			}
		}
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

	for tenant, envs := range envs {
		for env := range envs {
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
