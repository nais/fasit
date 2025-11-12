package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type (
	tenant          = string
	environmentName = string
	envSpec         struct {
		kind   protogen.EnvironmentKind
		labels environment.Labels
	}
)

var (
	naisProjectID      = "nais-local-dev"
	statusSubscription = "fasit-subscription"
	statusTopic        = "status"

	envs = map[tenant]map[environmentName]envSpec{
		"test-partner": {
			"dev": envSpec{
				kind: protogen.EnvironmentKind_TENANT,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"management": envSpec{
				kind: protogen.EnvironmentKind_MANAGEMENT,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
		},
		/*
			"nav": {
				"management": envSpec{
					kind:   protogen.EnvironmentKind_MANAGEMENT,
					labels: environment.Labels{},
				},
				"dev": envSpec{
					kind: protogen.EnvironmentKind_TENANT,
					labels: environment.Labels{
						"aiven": "enabled",
					},
				},
				"prod": envSpec{
					kind: protogen.EnvironmentKind_TENANT,
					labels: environment.Labels{
						"aiven": "enabled",
					},
				},
				"dev-fss": envSpec{
					kind: protogen.EnvironmentKind_ONPREM,
					labels: environment.Labels{
						"aiven": "enabled",
					},
				},
				"prod-fss": envSpec{
					kind: protogen.EnvironmentKind_ONPREM,
					labels: environment.Labels{
						"aiven": "enabled",
					},
				},
				"dev-gcp": envSpec{
					kind: protogen.EnvironmentKind_LEGACY,
					labels: environment.Labels{
						"aiven": "enabled",
					},
				},
				"prod-gcp": envSpec{
					kind: protogen.EnvironmentKind_LEGACY,
					labels: environment.Labels{
						"aiven": "enabled",
					},
				},
			},
			"ssb": {
				"dev": envSpec{
					kind:   protogen.EnvironmentKind_TENANT,
					labels: environment.Labels{},
				},
				"management": envSpec{
					kind:   protogen.EnvironmentKind_MANAGEMENT,
					labels: environment.Labels{},
				},
			},
			"dev-nais": {
				"dev": envSpec{
					kind:   protogen.EnvironmentKind_TENANT,
					labels: environment.Labels{},
				},
				"management": envSpec{
					kind:   protogen.EnvironmentKind_MANAGEMENT,
					labels: environment.Labels{},
				},
			},

		*/
	}

	deployments = []Deployment{
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "aivenator", Version: "1.0.0", Target: environment.Labels{"aiven": "enabled"}, Hash: "TODO"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "aivenator", Version: "2.0.0", Target: environment.Labels{"aiven": "enabled"}, Hash: "TODO"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "aivenator", Version: "1.0.0", Target: environment.Labels{"aiven": "enabled", "tenant": "nav"}, Hash: "TODO"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "naiserator", Version: "1.0.0", Target: environment.Labels{"aiven": "enabled"}, Hash: "TODO"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "aivenator", Version: "3.0.0", Target: environment.Labels{"aiven": "enabled"}, Hash: "TODO"},
			Dependencies:           []string{"naiserator"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "unleash", Version: "1.0.0", Target: environment.Labels{"featuretoggle": "enabled"}, Hash: "TODO"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "unleash", Version: "2.0.0", Target: environment.Labels{"featuretoggle": "enabled"}, Hash: "TODO"},
		},
		{
			DeploymentCreateParams: &gensql.DeploymentCreateParams{FeatureName: "v13s", Version: "1.0.0", Target: environment.Labels{"kind": "management"}, Hash: "TODO"},
		},
	}
)

type Deployment struct {
	*gensql.DeploymentCreateParams
	Dependencies []string
}

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	dbConn, cancel, err := database.NewDB(ctx, "postgres://postgres:postgres@localhost:5432/fasit?sslmode=disable", false)
	if err != nil {
		panic(err)
	}
	defer cancel.Close()

	conn, err := grpc.NewClient("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	grpcClient := protogen.NewProviderClient(conn)

	db := database.New(dbConn, logrus.New().WithField("component", "setup-local"))
	for _, d := range deployments {
		var deps model.Dependencies
		if len(d.Dependencies) > 0 {
			deps = model.Dependencies{
				&model.Dependency{
					AllOf: d.Dependencies,
				},
			}
		}

		_ = db.FeatureDataCreate(ctx, model.Feature{
			Name:    d.FeatureName,
			Version: d.Version,
			Chart:   "oci://aiven",
			FeatureYAML: model.FeatureYAML{
				Dependencies:     deps,
				EnvironmentKinds: []model.EnvironmentKind{"tenant", "management"},
			},
		}, nil)
		_, err = db.DeploymentCreate(ctx, d.FeatureName, d.Version, d.GhRef, d.Target, d.Hash)
		if err != nil {
			panic(err)
		}
	}
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
		for env, spec := range environments {
			lbls := spec.labels
			lbls["kind"] = strings.ToLower(spec.kind.String())
			lbls["environment"] = env
			lbls["tenant"] = tenantName

			_, err := grpcClient.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
				TenantId: tenant.Id,
				Name:     env,
				Kind:     spec.kind,
				Labels: func(lbls environment.Labels) []*protogen.EnvironmentLabel {
					out := make([]*protogen.EnvironmentLabel, 0)
					for k, v := range lbls {
						out = append(out, &protogen.EnvironmentLabel{Key: k, Value: v})
					}
					return out
				}(lbls),
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

	topicRes, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/" + naisProjectID + "/topics/" + statusTopic,
	})
	if err != nil {
		log.Println(err)
	}
	_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:  "projects/" + naisProjectID + "/subscriptions/" + statusSubscription,
		Topic: topicRes.GetName(),
	})
	if err != nil {
		log.Println(err)
	}

	for tenant, envs := range envs {
		for env := range envs {
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
		}
	}
}
