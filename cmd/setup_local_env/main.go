package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric/noop"
)

type (
	tenant          = string
	environmentName = string
	envSpec         struct {
		kind   model.EnvironmentKind
		labels environment.Labels
	}
)

var (
	naisProjectID      = "nais-local-dev"
	statusSubscription = "fasit-subscription"
	statusTopic        = "status"

	envs = map[tenant]map[environmentName]envSpec{
		"test-partner": {
			"management": {
				kind: model.EnvironmentKindManagement,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"dev": {
				kind: model.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"prod": {
				kind: model.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"staging": {
				kind:   model.EnvironmentKindTenant,
				labels: environment.Labels{},
			},
		},
		"nav": {
			"management": {
				kind:   model.EnvironmentKindManagement,
				labels: environment.Labels{},
			},
			"dev": {
				kind: model.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"prod": {
				kind: model.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
		},
		"dev-nais": {
			"management": {
				kind:   model.EnvironmentKindManagement,
				labels: environment.Labels{},
			},
			"dev": {
				kind:   model.EnvironmentKindTenant,
				labels: environment.Labels{},
			},
		},
	}
)

func main() {
	log := logrus.StandardLogger()

	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	dbConn, cancel, err := database.NewConnPool(ctx, "postgres://postgres:postgres@localhost:5432/fasit?sslmode=disable", log)
	if err != nil {
		panic(err)
	}
	defer cancel.Close()

	loadContext, err := contextloader.NewLoaderFunc(dbConn, nil, nil, noop.NewMeterProvider().Meter(""), logrus.New())
	if err != nil {
		panic(err)
	}
	ctx = loadContext(ctx)
	// Identify the seeder as a system actor so audit log entries don't warn
	// about an "unknown actor" for every feature_state we create below.
	ctx = auth.SetEmail(ctx, "setup_local_env")

	// Create tenants and environments first (needed for deployment status FKs).
	for tenantName, environments := range envs {
		_, err := environment.CreateTenant(ctx, &model.TenantCreate{Name: tenantName})
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				log.Fatal(err)
			}
		}

		tenant, err := environment.GetTenantGetByName(ctx, tenantName)
		if err != nil {
			log.Fatal(err)
		}

		for env, spec := range environments {
			lbls := spec.labels
			lbls["kind"] = strings.ToLower(spec.kind.String())
			lbls["environment"] = env
			lbls["tenant"] = tenantName

			e, err := environment.Create(ctx, &model.EnvironmentCreate{
				Name:     env,
				TenantID: tenant.ID,
				Kind:     spec.kind,
			})
			if err != nil {
				if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					continue
				}

				log.Fatal(err)
			}

			err = environment.SetLabels(ctx, e.ID, lbls)
			if err != nil {
				if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					continue
				}

				log.Fatal(err)
			}

			err = environment.SetEnvironmentValue(ctx, e.ID, "project_id", json.RawMessage(fmt.Sprintf("%q", "nais-"+env)), false)
			if err != nil {
				log.Fatal(err)
			}

			err = environment.SetEnvironmentValue(ctx, e.ID, "updated_at", json.RawMessage(fmt.Sprintf("%q", time.Now().String())), false)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	repo := database.NewRepo(dbConn, log)

	envID := func(tenantName, envName string) uuid.UUID {
		id, err := repo.EnvironmentIDByNames(ctx, tenantName, envName)
		if err != nil {
			log.WithField("tenant", tenantName).WithField("env", envName).Fatal(err)
		}
		return id
	}

	seeder := deploymenttest.NewSeeder()
	deployment.ChartDownloader = seeder.ChartDownloader()

	// Targets are chosen so that, when paired with mise/tasks/naisd-all.sh
	// (test-partner/prod runs --mock-failing, test-partner/staging has no
	// naisd at all), the local UI ends up with a representative mix:
	//   - 4 features successfully DEPLOYED
	//   - 2 features FAILED (touching test-partner/prod)
	//   - 2 features PENDING (touching test-partner/staging, which never
	//     receives a status because no naisd is listening there)
	tenantOnly := []model.EnvironmentKind{"tenant"}
	managementOnly := []model.EnvironmentKind{"management"}
	onpremOnly := []model.EnvironmentKind{"onprem"}
	all := append(append(tenantOnly, managementOnly...), onpremOnly...)

	str := &model.Config{Type: model.ConfigTypeString}
	intCfg := &model.Config{Type: model.ConfigTypeInt}
	boolCfg := &model.Config{Type: model.ConfigTypeBool}
	secret := &model.Config{Type: model.ConfigTypeString, Secret: true}

	featureValues := map[string]model.Values{
		"naiserator": {
			"replicas": {DisplayName: "Replicas", Description: "Number of replicas", Config: intCfg},
			"logLevel": {DisplayName: "Log Level", Config: str},
			"apiKey":   {DisplayName: "API Key", Description: "External API key", Config: secret},
		},
		"console": {
			"adminEmail":    {DisplayName: "Admin Email", Config: str},
			"sessionSecret": {DisplayName: "Session Secret", Config: secret},
			"debugMode":     {DisplayName: "Debug Mode", Config: boolCfg},
		},
		"unleash": {
			"instanceCount": {DisplayName: "Instance Count", Config: intCfg},
			"dbPassword":    {DisplayName: "Database Password", Config: secret},
		},
		"replicator": {
			"syncInterval": {DisplayName: "Sync Interval", Description: "Seconds between syncs", Config: str},
			"maxRetries":   {DisplayName: "Max Retries", Config: intCfg},
		},
		"v13s": {
			"clusterName": {DisplayName: "Cluster Name", Config: str},
			"dryRun":      {DisplayName: "Dry Run", Config: boolCfg},
		},
		"dependencytrack": {
			"apiUrl":   {DisplayName: "API URL", Config: str},
			"apiToken": {DisplayName: "API Token", Config: secret},
		},
		"kyverno": {
			"webhookTimeout": {DisplayName: "Webhook Timeout", Description: "Timeout in seconds", Config: intCfg},
			"replicaCount":   {DisplayName: "Replica Count", Config: intCfg},
		},
		"aivenator": {
			"aivenToken":  {DisplayName: "Aiven Token", Config: secret},
			"projectName": {DisplayName: "Project Name", Config: str},
		},
		"hookd": {
			"webhookUrl":    {DisplayName: "Webhook URL", Config: str},
			"webhookSecret": {DisplayName: "Webhook Secret", Config: secret},
		},
	}

	seeder.AddDeploymentWithValues("naiserator", "2026-04-28-a1b2c3d", environment.Labels{"kind": "tenant"}, tenantOnly, featureValues["naiserator"])
	seeder.AddDeploymentWithValues("v13s", "2026-04-22-7e8f9a0", environment.Labels{"kind": "management"}, managementOnly, featureValues["v13s"])
	seeder.AddDeploymentWithValues("console", "2026-04-30-3c4d5e6", environment.Labels{"kind": "management"}, managementOnly, featureValues["console"])
	seeder.AddDeploymentWithValues("unleash", "2026-04-15-9b0c1d2", environment.Labels{"kind": "management", "aiven": "enabled"}, managementOnly, featureValues["unleash"])
	seeder.AddDeploymentWithValues("replicator", "2026-04-18-5f6a7b8", environment.Labels{"kind": "tenant", "tenant": "test-partner", "environment": "prod"}, tenantOnly, featureValues["replicator"])
	seeder.AddDeploymentWithValues("dependencytrack", "2026-04-10-2d3e4f5", environment.Labels{"kind": "tenant", "tenant": "test-partner", "environment": "staging"}, tenantOnly, featureValues["dependencytrack"])
	seeder.AddDeploymentWithValues("naiserator", "2026-05-01-6a7b8c9", environment.Labels{"kind": "tenant", "tenant": "dev-nais", "environment": "dev"}, tenantOnly, featureValues["naiserator"])
	seeder.AddDeploymentWithValues("dependencytrack", "2026-04-25-8d9e0f1", environment.Labels{"kind": "tenant", "tenant": "dev-nais", "environment": "dev"}, tenantOnly, featureValues["dependencytrack"])
	seeder.AddDeploymentWithValues("replicator", "2026-04-20-4b5c6d7", environment.Labels{"kind": "tenant", "tenant": "dev-nais", "environment": "dev"}, tenantOnly, featureValues["replicator"])
	seeder.AddDeploymentWithValues("unleash", "2026-04-27-1a2b3c4", environment.Labels{"kind": "management", "tenant": "dev-nais"}, managementOnly, featureValues["unleash"])
	seeder.AddDeploymentWithValues("kyverno", "2026-04-27-1a2b3c5", environment.Labels{}, all, featureValues["kyverno"])

	if _, err := seeder.Seed(ctx); err != nil {
		log.Fatal(err)
	}

	// Enable every feature in every env so they don't default to disabled.
	for tenantName, environments := range envs {
		for envName := range environments {
			id := envID(tenantName, envName)
			states, err := feature.FeatureStatesGet(ctx, id)
			if err != nil {
				log.WithError(err).Errorf("get feature states for %s/%s", tenantName, envName)
				continue
			}
			for _, state := range states {
				if _, err := feature.FeatureStatesCreateOrUpdate(ctx, id, &model.Feature{Name: state.FeatureName}, true); err != nil {
					log.WithError(err).Errorf("enable %s in %s/%s", state.FeatureName, tenantName, envName)
				}
			}
		}
	}

	// Disable one feature to get a DISABLED status row in the mix.
	if _, err := feature.FeatureStatesCreateOrUpdate(ctx, envID("dev-nais", "dev"), &model.Feature{Name: "dependencytrack"}, false); err != nil {
		log.WithError(err).Errorf("disable dependencytrack in dev-nais/dev")
	}

	type rolloutSeed struct {
		name    string
		version string
		ref     string
	}
	rollouts := []rolloutSeed{
		{"naiserator", "2026-05-04-f1e2d3c", "refs/heads/main"},
		{"aivenator", "1.0.0", "refs/tags/v1.0.0"},
		{"aivenator", "2.0.0", "refs/tags/v2.0.0"},
		{"aivenator", "3.0.0", "refs/tags/v3.0.0"},
		{"console", "2.0.0", "refs/heads/main"},
		{"hookd", "1.0.0", "refs/tags/v1.0.0"},
		{"dependencytrack", "2.0.0", "refs/heads/main"},
		{"replicator", "2.0.0", "refs/tags/v2.0.0"},
	}

	rolloutSeedKinds := map[string][]model.EnvironmentKind{
		"naiserator":      tenantOnly,
		"aivenator":       tenantOnly,
		"console":         managementOnly,
		"hookd":           managementOnly,
		"dependencytrack": managementOnly,
		"replicator":      tenantOnly,
	}

	for _, r := range rollouts {
		kinds := rolloutSeedKinds[r.name]
		if kinds == nil {
			kinds = []model.EnvironmentKind{"tenant", "management"}
		}
		feat := model.Feature{
			Name:    r.name,
			Version: r.version,
			Chart:   "oci://" + r.name,
			Source:  "https://example.com/" + r.name,
			FeatureYAML: model.FeatureYAML{
				EnvironmentKinds: kinds,
				Values:           featureValues[r.name],
			},
		}
		if err := feature.FeatureDataCreate(ctx, feat, nil); err != nil {
			if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				log.WithError(err).Warnf("failed to seed feature_data for %s %s", r.name, r.version)
				continue
			}
		}

		_, err := repo.RolloutCreate(ctx, r.name, r.version, &model.GHRef{Owner: "nais", Repo: r.name, Ref: r.ref})
		if err != nil {
			log.WithError(err).Warnf("failed to create rollout for %s %s", r.name, r.version)
		}
	}

	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, "naiserator", true); err != nil {
		log.WithError(err).Warn("failed to update naiserator rollout status")
	}
	if err := feature.FeatureVersionUpdate(ctx, "naiserator", "2026-05-04-f1e2d3c"); err != nil {
		log.WithError(err).Warn("failed to update naiserator feature version")
	}
	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, "aivenator", true); err != nil {
		log.WithError(err).Warn("failed to update aivenator rollout status")
	}
	if err := feature.FeatureVersionUpdate(ctx, "aivenator", "3.0.0"); err != nil {
		log.WithError(err).Warn("failed to update aivenator feature version")
	}
	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusFailed, "hookd", false); err != nil {
		log.WithError(err).Warn("failed to update hookd rollout status")
	}
	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, "console", true); err != nil {
		log.WithError(err).Warn("failed to update console rollout status")
	}
	if err := feature.FeatureVersionUpdate(ctx, "console", "2.0.0"); err != nil {
		log.WithError(err).Warn("failed to update console feature version")
	}

	// Set up pubsub topics and subscriptions.
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
