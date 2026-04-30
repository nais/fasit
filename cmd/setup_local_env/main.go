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
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
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

	// [0] aivenator: targets {aiven:enabled} → matches dev, management, prod (not staging)
	seeder.AddDeployment("aivenator", "2.0.0", environment.Labels{"aiven": "enabled"})
	// [1] naiserator: targets {kind:tenant} → matches dev, prod, staging (not management)
	seeder.AddDeployment("naiserator", "1.0.0", environment.Labels{"kind": "tenant"})
	// [2] v13s: targets {kind:management} → matches management only
	seeder.AddDeployment("v13s", "1.0.0", environment.Labels{"kind": "management"})
	// [3] console: global (empty target) → matches all environments
	seeder.AddDeployment("console", "1.0.0", environment.Labels{})
	// [4] unleash: targets {aiven:enabled} → matches dev, management, prod; dev will be DISABLED via feature_state
	seeder.AddDeployment("unleash", "1.0.0", environment.Labels{"aiven": "enabled"})
	// [5] kafka-manager: targets {kafka:enabled} → matches no environments
	seeder.AddDeployment("kafka-manager", "1.0.0", environment.Labels{"kafka": "enabled"})
	// [6] naiserator: targets {kind:tenant, tenant:nav} → more specific, overrides [1] for nav's tenant envs
	seeder.AddDeployment("naiserator", "1.1.0", environment.Labels{"kind": "tenant", "tenant": "nav"})
	// [7] naiserator: targets {kind:tenant, aiven:enabled, tenant:test-partner} → most specific, overrides [1] for test-partner dev+prod
	seeder.AddDeployment("naiserator", "1.2.0", environment.Labels{"kind": "tenant", "aiven": "enabled", "tenant": "test-partner"})

	depIDs, err := seeder.Seed(ctx)
	if err != nil {
		log.Fatal(err)
	}

	setStatus := func(depIdx int, tenantName, envName string, status model.RolloutStatus, msg string) {
		if err := deployment.SetDeploymentStatus(ctx, depIDs[depIdx], envID(tenantName, envName), status, msg); err != nil {
			log.WithError(err).WithField("deployment", depIdx).WithField("env", envName).Error("set deployment status")
		}
	}

	// aivenator [0]: targets {aiven:enabled}
	setStatus(0, "test-partner", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(0, "test-partner", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(0, "test-partner", "prod", model.RolloutStatusFailed, "helm install failed: timeout waiting for condition")
	setStatus(0, "nav", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(0, "nav", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(0, "nav", "prod", model.RolloutStatusDeployed, "received status from naisd.")

	// naiserator [1]: targets {kind:tenant}
	setStatus(1, "test-partner", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(1, "test-partner", "prod", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(1, "test-partner", "staging", model.RolloutStatusPending, "waiting for naisd")
	setStatus(1, "nav", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(1, "nav", "prod", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(1, "dev-nais", "dev", model.RolloutStatusDeployed, "received status from naisd.")

	// v13s [2]: targets {kind:management}
	setStatus(2, "test-partner", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(2, "nav", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(2, "dev-nais", "management", model.RolloutStatusDeployed, "received status from naisd.")

	// console [3]: global
	setStatus(3, "test-partner", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "test-partner", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "test-partner", "prod", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "test-partner", "staging", model.RolloutStatusCreated, "deployment instruction sent to naisd")
	setStatus(3, "nav", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "nav", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "nav", "prod", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "dev-nais", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(3, "dev-nais", "management", model.RolloutStatusDeployed, "received status from naisd.")

	// unleash [4]: targets {aiven:enabled}
	setStatus(4, "test-partner", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(4, "test-partner", "prod", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(4, "nav", "management", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(4, "nav", "prod", model.RolloutStatusDeployed, "received status from naisd.")

	// naiserator [6]: targets {kind:tenant, tenant:nav}
	setStatus(6, "nav", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(6, "nav", "prod", model.RolloutStatusDeployed, "received status from naisd.")

	// naiserator [7]: targets {kind:tenant, aiven:enabled, tenant:test-partner}
	setStatus(7, "test-partner", "dev", model.RolloutStatusDeployed, "received status from naisd.")
	setStatus(7, "test-partner", "prod", model.RolloutStatusFailed, "helm install failed: timeout waiting for condition")

	// Disable unleash in dev so ListDeploymentStatuses synthesizes a DISABLED row.
	if _, err := feature.FeatureStatesCreateOrUpdate(ctx, envID("test-partner", "dev"), &model.Feature{Name: "unleash"}, false); err != nil {
		log.WithError(err).Error("disable unleash in dev")
	}

	setRelease := func(tenantName, envName, featureName, version, status string) {
		if err := repo.ReleaseStatusCreateOrUpdate(ctx, envID(tenantName, envName), &message.Release{
			Name:         featureName,
			Version:      version,
			Status:       status,
			Revision:     1,
			LastDeployed: time.Now().Add(-1 * time.Hour),
		}); err != nil {
			log.WithError(err).WithField("tenant", tenantName).WithField("env", envName).WithField("feature", featureName).Error("set release status")
		}
	}

	// aivenator: desired 2.0.0 — prod failed, still on old version
	setRelease("test-partner", "dev", "aivenator", "2.0.0", "deployed")
	setRelease("test-partner", "management", "aivenator", "2.0.0", "deployed")
	setRelease("test-partner", "prod", "aivenator", "1.0.0", "deployed")
	setRelease("nav", "dev", "aivenator", "2.0.0", "deployed")
	setRelease("nav", "management", "aivenator", "2.0.0", "deployed")
	setRelease("nav", "prod", "aivenator", "2.0.0", "deployed")

	// naiserator: desired 1.0.0 — staging pending (no release yet)
	setRelease("test-partner", "dev", "naiserator", "1.2.0", "deployed")
	setRelease("test-partner", "prod", "naiserator", "1.0.0", "deployed")
	setRelease("nav", "dev", "naiserator", "1.1.0", "deployed")
	setRelease("nav", "prod", "naiserator", "1.1.0", "deployed")
	setRelease("dev-nais", "dev", "naiserator", "1.0.0", "deployed")

	// v13s: desired 1.0.0
	setRelease("test-partner", "management", "v13s", "1.0.0", "deployed")
	setRelease("nav", "management", "v13s", "1.0.0", "deployed")
	setRelease("dev-nais", "management", "v13s", "1.0.0", "deployed")

	// console: desired 1.0.0 — staging still pending (no release yet)
	setRelease("test-partner", "dev", "console", "1.0.0", "deployed")
	setRelease("test-partner", "management", "console", "1.0.0", "deployed")
	setRelease("test-partner", "prod", "console", "1.0.0", "deployed")
	setRelease("nav", "dev", "console", "1.0.0", "deployed")
	setRelease("nav", "management", "console", "1.0.0", "deployed")
	setRelease("nav", "prod", "console", "1.0.0", "deployed")
	setRelease("dev-nais", "dev", "console", "1.0.0", "deployed")
	setRelease("dev-nais", "management", "console", "1.0.0", "deployed")

	// unleash: desired 1.0.0
	setRelease("test-partner", "management", "unleash", "1.0.0", "deployed")
	setRelease("test-partner", "prod", "unleash", "1.0.0", "deployed")
	setRelease("nav", "management", "unleash", "1.0.0", "deployed")
	setRelease("nav", "prod", "unleash", "1.0.0", "deployed")

	type rolloutSeed struct {
		name    string
		version string
		ref     string
	}
	rollouts := []rolloutSeed{
		{"naiserator", "1.0.0", "refs/heads/main"},
		{"aivenator", "1.0.0", "refs/tags/v1.0.0"},
		{"aivenator", "2.0.0", "refs/tags/v2.0.0"},
		{"aivenator", "3.0.0", "refs/tags/v3.0.0"},
		{"console", "2.0.0", "refs/heads/main"},
		{"hookd", "1.0.0", "refs/tags/v1.0.0"},
		{"dependencytrack", "2.0.0", "refs/heads/main"},
		{"replicator", "2.0.0", "refs/tags/v2.0.0"},
	}

	for _, r := range rollouts {
		_, err := repo.RolloutCreate(ctx, r.name, r.version, &model.GHRef{Owner: "nais", Repo: r.name, Ref: r.ref})
		if err != nil {
			log.WithError(err).Warnf("failed to create rollout for %s %s", r.name, r.version)
		}
	}

	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, "naiserator", true); err != nil {
		log.WithError(err).Warn("failed to update naiserator rollout status")
	}
	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, "aivenator", true); err != nil {
		log.WithError(err).Warn("failed to update aivenator rollout status")
	}
	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusFailed, "hookd", false); err != nil {
		log.WithError(err).Warn("failed to update hookd rollout status")
	}
	if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, "console", true); err != nil {
		log.WithError(err).Warn("failed to update console rollout status")
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
