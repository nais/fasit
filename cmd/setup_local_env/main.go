package main

import (
	"context"
	"crypto/rand"
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
	now := time.Now().UTC().Format("2006-01-02-150405")
	seq := 0
	newVersion := func() string {
		seq++
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		return fmt.Sprintf("%s-%x%d", now, b, seq)
	}

	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	dbConn, cancel, err := database.NewConnPool(ctx, "postgres://postgres:postgres@localhost:5432/fasit?sslmode=disable", log)
	if err != nil {
		panic(err)
	}
	defer cancel.Close()

	loadContext, err := contextloader.NewLoaderFunc(dbConn, nil, noop.NewMeterProvider().Meter(""), logrus.New())
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

			// Secret env values: consumed by computed templates in several features
			// below. Seeded on every environment so the env-config view can show
			// the new secret-taint masking in action.
			secretEnvValues := map[string]string{
				"db_password": "s3cr3t-" + env,
				"slack_token": "xoxb-" + env + "-token",
				"api_key":     "ak_live_" + env + "_xyz",
			}
			for k, v := range secretEnvValues {
				if err := environment.SetEnvironmentValue(ctx, e.ID, k, json.RawMessage(fmt.Sprintf("%q", v)), true); err != nil {
					log.Fatal(err)
				}
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
	strArr := &model.Config{Type: model.ConfigTypeStringArray}

	// Feature catalog. Each feature has 5–10 values mixing:
	//   - plain configs (some Required)
	//   - secret configs (Secret: true)
	//   - computed values that reference non-secret data
	//   - computed values that reference secret env values or secret configs
	//     (these are the *computed secrets* the env-config view masks)
	featureValues := map[string]model.Values{
		"naiserator": {
			"replicas":      {DisplayName: "Replicas", Description: "Number of replicas", Required: true, Config: intCfg},
			"logLevel":      {DisplayName: "Log Level", Config: str},
			"apiKey":        {DisplayName: "API Key", Description: "External API key", Required: true, Config: secret},
			"clusterDomain": {DisplayName: "Cluster Domain", Description: "Derived from environment name", Computed: &model.Computed{Template: `"{{ .Env.name }}.{{ .Tenant.Name }}.cloud.nais.io"`}},
			"projectRef":    {DisplayName: "GCP Project Ref", Computed: &model.Computed{Template: `"projects/{{ .Env.project_id }}"`}},
			"imageTag":      {DisplayName: "Image Tag", Description: "Override image tag; falls back to a computed default", Config: str, Computed: &model.Computed{Template: `"{{ .Env.name }}-latest"`}},
			"featureFlags":  {DisplayName: "Feature Flags", Description: "JSON blob of toggles", Config: str},
			"extraEnv":      {DisplayName: "Extra Env", Description: "Additional KEY=VALUE pairs", Config: strArr},
			"motd":          {DisplayName: "Message of the Day", Description: "Multi-line banner shown in the UI", Config: str},
			"allowedHosts":  {DisplayName: "Allowed Hosts", Description: "Required list; chart default is empty so warns until set", Required: true, Config: strArr},
			// Computed secret: reads the secret env value db_password.
			"dbDsn": {DisplayName: "DB DSN", Description: "Derived from secret env db_password", Computed: &model.Computed{Template: `"postgres://naiserator:{{ .Env.db_password }}@db.{{ .Env.name }}.local/naiserator"`}},
		},
		"console": {
			"adminEmail":       {DisplayName: "Admin Email", Required: true, Config: str},
			"sessionSecret":    {DisplayName: "Session Secret", Config: secret},
			"debugMode":        {DisplayName: "Debug Mode", Config: boolCfg},
			"port":             {DisplayName: "Port", Config: intCfg},
			"oauthClientId":    {DisplayName: "OAuth Client ID", Config: str},
			"baseUrl":          {DisplayName: "Base URL", Computed: &model.Computed{Template: `"https://console.{{ .Env.name }}.{{ .Tenant.Name }}.nais.io"`}},
			"oauthRedirectUri": {DisplayName: "OAuth Redirect URI", Computed: &model.Computed{Template: `"https://console.{{ .Env.name }}.{{ .Tenant.Name }}.nais.io/oauth/callback"`}},
			// Computed secret: reads the secret env value slack_token.
			"slackWebhook": {DisplayName: "Slack Webhook URL", Description: "Derived from secret env slack_token", Computed: &model.Computed{Template: `"https://hooks.slack.com/services/{{ .Env.slack_token }}"`}},
		},
		"unleash": {
			"instanceCount": {DisplayName: "Instance Count", Config: intCfg},
			"dbPassword":    {DisplayName: "Database Password", Required: true, Config: secret},
			"dbHost":        {DisplayName: "Database Host", Required: true, Config: str},
			"dbName":        {DisplayName: "Database Name", Config: str},
			"adminToken":    {DisplayName: "Admin Token", Config: secret},
			"metricsPath":   {DisplayName: "Metrics Path", Config: str},
			// Computed secret: reads secret config dbPassword.
			"dbUrl": {DisplayName: "Database URL", Description: "Derived from secret config dbPassword", Computed: &model.Computed{Template: `"postgres://unleash:{{ .Configs.dbPassword }}@{{ .Configs.dbHost }}/{{ .Configs.dbName }}"`}},
		},
		"replicator": {
			"syncInterval":  {DisplayName: "Sync Interval", Description: "Seconds between syncs", Config: str},
			"maxRetries":    {DisplayName: "Max Retries", Config: intCfg},
			"concurrency":   {DisplayName: "Concurrency", Required: true, Config: intCfg},
			"logLevel":      {DisplayName: "Log Level", Config: str},
			"targetCluster": {DisplayName: "Target Cluster", Computed: &model.Computed{Template: `"{{ .Env.name }}-replica"`}},
			// Computed secret: reads secret env api_key.
			"apiAuthHeader": {DisplayName: "API Auth Header", Description: "Derived from secret env api_key", Computed: &model.Computed{Template: `"Bearer {{ .Env.api_key }}"`}},
		},
		"v13s": {
			"clusterName":   {DisplayName: "Cluster Name", Required: true, Config: str},
			"dryRun":        {DisplayName: "Dry Run", Config: boolCfg},
			"scanInterval":  {DisplayName: "Scan Interval", Config: intCfg},
			"imageRegistry": {DisplayName: "Image Registry", Required: true, Config: str},
			"dbPassword":    {DisplayName: "Database Password", Config: secret},
			"apiEndpoint":   {DisplayName: "API Endpoint", Computed: &model.Computed{Template: `"https://v13s.{{ .Env.name }}.nais.io"`}},
			// Computed secret: reads secret config dbPassword.
			"dbUrl": {DisplayName: "Database URL", Description: "Derived from secret config dbPassword", Computed: &model.Computed{Template: `"postgres://v13s:{{ .Configs.dbPassword }}@db.{{ .Env.name }}/v13s"`}},
		},
		"dependencytrack": {
			"apiUrl":          {DisplayName: "API URL", Required: true, Config: str},
			"apiToken":        {DisplayName: "API Token", Config: secret},
			"adminEmail":      {DisplayName: "Admin Email", Config: str},
			"pollInterval":    {DisplayName: "Poll Interval", Config: intCfg},
			"notificationUrl": {DisplayName: "Notification URL", Computed: &model.Computed{Template: `"https://dt.{{ .Env.name }}.{{ .Tenant.Name }}.nais.io/notify"`}},
			// Computed secret: reads secret env slack_token.
			"slackAlertUrl": {DisplayName: "Slack Alert URL", Description: "Derived from secret env slack_token", Computed: &model.Computed{Template: `"https://hooks.slack.com/services/{{ .Env.slack_token }}/alerts"`}},
		},
		"kyverno": {
			"webhookTimeout":    {DisplayName: "Webhook Timeout", Description: "Timeout in seconds", Config: intCfg},
			"replicaCount":      {DisplayName: "Replica Count", Required: true, Config: intCfg},
			"webhookSigningKey": {DisplayName: "Webhook Signing Key", Config: secret},
			"webhookURL":        {DisplayName: "Webhook URL", Computed: &model.Computed{Template: `"https://hooks.{{ .Env.name }}.{{ .Tenant.Name }}.example.com/kyverno"`}},
			"envKind":           {DisplayName: "Environment Kind", Computed: &model.Computed{Template: `"{{ .Env.kind }}"`}},
			// Computed secret: reads secret config webhookSigningKey.
			"signedWebhookURL": {DisplayName: "Signed Webhook URL", Description: "Derived from secret config webhookSigningKey", Computed: &model.Computed{Template: `"https://hooks.{{ .Env.name }}.{{ .Tenant.Name }}.example.com/kyverno?sig={{ .Configs.webhookSigningKey }}"`}},
		},
		"aivenator": {
			"aivenToken":   {DisplayName: "Aiven Token", Required: true, Config: secret},
			"projectName":  {DisplayName: "Project Name", Required: true, Config: str},
			"region":       {DisplayName: "Region", Config: str},
			"adminEmail":   {DisplayName: "Admin Email", Config: str},
			"serviceUrl":   {DisplayName: "Service URL", Computed: &model.Computed{Template: `"https://aiven.{{ .Env.name }}.nais.io"`}},
			"dashboardUrl": {DisplayName: "Dashboard URL", Computed: &model.Computed{Template: `"https://aiven.{{ .Env.name }}.nais.io/dashboard"`}},
			// Computed secret: reads secret env api_key.
			"apiAuthHeader": {DisplayName: "API Auth Header", Description: "Derived from secret env api_key", Computed: &model.Computed{Template: `"Bearer {{ .Env.api_key }}"`}},
		},
		"hookd": {
			"webhookUrl":    {DisplayName: "Webhook URL", Required: true, Config: str},
			"webhookSecret": {DisplayName: "Webhook Secret", Config: secret},
			"port":          {DisplayName: "Port", Config: intCfg},
			"githubAppId":   {DisplayName: "GitHub App ID", Required: true, Config: str},
			"slackChannel":  {DisplayName: "Slack Channel", Config: str},
			// Computed secret: reads secret config webhookSecret.
			"signedCallback": {DisplayName: "Signed Callback URL", Description: "Derived from secret config webhookSecret", Computed: &model.Computed{Template: `"{{ .Configs.webhookUrl }}?sig={{ .Configs.webhookSecret }}"`}},
		},
	}

	featureDefaults := map[string]map[string]any{
		"naiserator": {
			"replicas":     2,
			"logLevel":     "info",
			"featureFlags": `{"experimentalA":true,"rolloutPercent":25,"regions":["eu","us"]}`,
			"extraEnv":     []string{"FOO=bar", "BAZ=qux"},
			"motd":         "line one\nline two\nline three",
			"allowedHosts": []string{},
		},
		"console": {
			"adminEmail":    "admin@example.com",
			"debugMode":     false,
			"port":          3000,
			"oauthClientId": "console-local",
		},
		"unleash": {
			"instanceCount": 1,
			"dbHost":        "db.unleash.local",
			"dbName":        "unleash",
			"metricsPath":   "/metrics",
		},
		"replicator": {
			"syncInterval": "60",
			"maxRetries":   3,
			"concurrency":  4,
			"logLevel":     "info",
		},
		"v13s": {
			"dryRun":        false,
			"scanInterval":  300,
			"imageRegistry": "europe-north1-docker.pkg.dev/nais",
		},
		"dependencytrack": {
			"apiUrl":       "https://dt.example.com",
			"adminEmail":   "dt-admin@example.com",
			"pollInterval": 600,
		},
		"kyverno": {
			"webhookTimeout": 10,
			"replicaCount":   3,
		},
		"aivenator": {
			"projectName": "nais-aiven",
			"region":      "europe-north1",
			"adminEmail":  "aiven-admin@example.com",
		},
		"hookd": {
			"webhookUrl":   "https://hookd.example.com/webhook",
			"port":         8080,
			"githubAppId":  "123456",
			"slackChannel": "#deploys",
		},
	}

	addDeployment := func(name, version string, target environment.Labels, kinds []model.EnvironmentKind) {
		seeder.AddDeploymentWithValues(name, version, target, kinds, featureValues[name], featureDefaults[name])
	}

	naiseratorV := newVersion()
	v13sV := newVersion()
	consoleV := newVersion()
	unleashV := newVersion()
	replicatorV := newVersion()
	dependencytrackV := newVersion()
	naiseratorDevV := newVersion()
	dependencytrackDevV := newVersion()
	replicatorDevV := newVersion()
	unleashDevV := newVersion()
	kyvernoV := newVersion()

	addDeployment("naiserator", naiseratorV, environment.Labels{"kind": "tenant"}, tenantOnly)
	addDeployment("v13s", v13sV, environment.Labels{"kind": "management"}, managementOnly)
	addDeployment("console", consoleV, environment.Labels{"kind": "management"}, managementOnly)
	addDeployment("unleash", unleashV, environment.Labels{"kind": "management", "aiven": "enabled"}, managementOnly)
	addDeployment("replicator", replicatorV, environment.Labels{"kind": "tenant", "tenant": "test-partner", "environment": "prod"}, tenantOnly)
	addDeployment("dependencytrack", dependencytrackV, environment.Labels{"kind": "tenant", "tenant": "test-partner", "environment": "staging"}, tenantOnly)
	addDeployment("naiserator", naiseratorDevV, environment.Labels{"kind": "tenant", "tenant": "dev-nais", "environment": "dev"}, tenantOnly)
	addDeployment("dependencytrack", dependencytrackDevV, environment.Labels{"kind": "tenant", "tenant": "dev-nais", "environment": "dev"}, tenantOnly)
	addDeployment("replicator", replicatorDevV, environment.Labels{"kind": "tenant", "tenant": "dev-nais", "environment": "dev"}, tenantOnly)
	addDeployment("unleash", unleashDevV, environment.Labels{"kind": "management", "tenant": "dev-nais"}, managementOnly)
	addDeployment("kyverno", kyvernoV, environment.Labels{}, all)

	if _, err := seeder.Seed(ctx); err != nil {
		log.Fatal(err)
	}

	// Persist featureDefaults as configurations_global rows so that
	// Required-field validation passes for features that ship chart defaults
	// (the helm tab and deploy path both validate). In production these
	// would typically be set by an operator via the UI.
	for featureName, defaults := range featureDefaults {
		for key, val := range defaults {
			b, err := json.Marshal(val)
			if err != nil {
				log.WithError(err).Errorf("marshal default %s/%s", featureName, key)
				continue
			}
			// Skip empty values: they wouldn't satisfy required-field validation,
			// and seeding them as globals would mask the chart default in the UI,
			// preventing the required-but-unset warning from rendering.
			if isEmptyJSONValue(b) {
				continue
			}
			if _, err := feature.ConfigCreate(ctx, model.NewConfiguration{
				Feature: featureName,
				Key:     key,
				Value:   json.RawMessage(b),
			}); err != nil {
				if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					log.WithError(err).Errorf("seed global default %s/%s", featureName, key)
				}
			}
		}
	}

	// Features are enabled by default (absence from disabled_features = enabled).
	// Disable one feature to get a DISABLED status row in the mix.
	if err := feature.FeatureDisable(ctx, envID("dev-nais", "dev"), "dependencytrack", "seeded as disabled for local dev"); err != nil {
		log.WithError(err).Errorf("disable dependencytrack in dev-nais/dev")
	}

	// Sprinkle a couple of env-level config overrides on naiserator so the Config
	// tab shows a mix of overridden / default rows (exercising the override-first sort).
	overrides := []struct {
		tenant, env, feature, key string
		value                     string
	}{
		{"dev-nais", "dev", "naiserator", "replicas", `5`},
		{"dev-nais", "dev", "naiserator", "featureFlags", `"{\"experimentalA\":false,\"rolloutPercent\":100,\"regions\":[\"eu\"]}"`},
		{"test-partner", "prod", "naiserator", "logLevel", `"debug"`},
		{"test-partner", "prod", "naiserator", "apiKey", `"override-secret-naiserator-prod"`},
		{"dev-nais", "dev", "unleash", "dbHost", `"db-override.dev.local"`},
		{"test-partner", "dev", "console", "port", `4000`},
	}
	for _, o := range overrides {
		id := envID(o.tenant, o.env)
		if _, err := feature.ConfigCreate(ctx, model.NewConfiguration{
			EnvironmentID: &id,
			Feature:       o.feature,
			Key:           o.key,
			Value:         json.RawMessage(o.value),
		}); err != nil {
			log.WithError(err).Errorf("seed override %s/%s in %s/%s", o.feature, o.key, o.tenant, o.env)
		}
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

func isEmptyJSONValue(b []byte) bool {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return false
	}
	switch typed := v.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}
