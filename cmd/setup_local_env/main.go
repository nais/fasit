package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/ioconvenience"
)

type (
	tenant          = string
	environmentName = string
	envSpec         struct {
		kind   environment.EnvironmentKind
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
				kind: environment.EnvironmentKindManagement,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"dev": {
				kind: environment.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"prod": {
				kind: environment.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"staging": {
				kind:   environment.EnvironmentKindTenant,
				labels: environment.Labels{},
			},
		},
		"nav": {
			"management": {
				kind:   environment.EnvironmentKindManagement,
				labels: environment.Labels{},
			},
			"dev": {
				kind: environment.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
			"prod": {
				kind: environment.EnvironmentKindTenant,
				labels: environment.Labels{
					"aiven": "enabled",
				},
			},
		},
		"dev-nais": {
			"management": {
				kind:   environment.EnvironmentKindManagement,
				labels: environment.Labels{},
			},
			"dev": {
				kind:   environment.EnvironmentKindTenant,
				labels: environment.Labels{},
			},
		},
	}
)

func main() {
	log := slog.Default()
	now := time.Now().UTC().Format("2006-01-02-150405")
	seq := 0
	newVersion := func() string {
		seq++
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		return fmt.Sprintf("%s-%x%d", now, b, seq)
	}

	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086"); err != nil {
		log.With("err", err).Error("fatal")
		os.Exit(1)
	}

	ctx := context.Background()
	dbConn, cancel, err := database.NewConnPool(ctx, "postgres://postgres:postgres@localhost:5432/fasit?sslmode=disable", log)
	if err != nil {
		panic(err)
	}
	defer ioconvenience.CloseWithLog(cancel, log)

	loadContext, err := contextloader.NewLoaderFunc(dbConn, slog.Default())
	if err != nil {
		panic(err)
	}
	ctx = loadContext(ctx)
	// Identify the seeder as a system actor so audit log entries don't warn
	// about an "unknown actor" for every feature_state we create below.
	ctx = auth.SetEmail(ctx, "setup_local_env")

	// Create tenants and environments first (needed for deployment status FKs).
	for tenantName, environments := range envs {
		_, err := environment.CreateTenant(ctx, &environment.TenantCreate{Name: tenantName})
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				log.With("err", err).Error("fatal")
				os.Exit(1)
			}
		}

		tenant, err := environment.GetTenantByName(ctx, tenantName)
		if err != nil {
			log.With("err", err).Error("fatal")
			os.Exit(1)
		}

		for env, spec := range environments {
			lbls := spec.labels
			lbls["kind"] = strings.ToLower(spec.kind.String())

			e, err := environment.Create(ctx, &environment.EnvironmentCreate{
				Name:     env,
				TenantID: tenant.ID,
				Kind:     spec.kind,
				Labels:   lbls,
			})
			if err != nil {
				if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					continue
				}

				log.With("err", err).Error("fatal")
				os.Exit(1)
			}

			err = environment.SetEnvironmentValue(ctx, e.ID, "project_id", json.RawMessage(fmt.Sprintf("%q", "nais-"+env)), false)
			if err != nil {
				log.With("err", err).Error("fatal")
				os.Exit(1)
			}

			err = environment.SetEnvironmentValue(ctx, e.ID, "updated_at", json.RawMessage(fmt.Sprintf("%q", time.Now().String())), false)
			if err != nil {
				log.With("err", err).Error("fatal")
				os.Exit(1)
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
					log.With("err", err).Error("fatal")
					os.Exit(1)
				}
			}
		}
	}

	envID := func(tenantName, envName string) uuid.UUID {
		t, err := environment.GetTenantByName(ctx, tenantName)
		if err != nil {
			log.With("err", err).Error("fatal")
			os.Exit(1)
		}
		env, err := environment.GetByName(ctx, t.ID, envName)
		if err != nil {
			log.With("err", err, "tenant", tenantName, "env", envName).Error("fatal")
			os.Exit(1)
		}
		return env.ID
	}

	seeder := featureassignmenttest.NewSeeder()
	featureassignment.ChartDownloader = seeder.ChartDownloader()

	// Targets are chosen so that, when paired with mise/tasks/naisd-all.sh
	// (test-partner/prod runs --mock-failing, test-partner/staging has no
	// naisd at all), the local UI ends up with a representative mix:
	//   - 4 features successfully DEPLOYED
	//   - 2 features FAILED (touching test-partner/prod)
	//   - 2 features PENDING (touching test-partner/staging, which never
	//     receives a status because no naisd is listening there)
	tenantOnly := []environment.EnvironmentKind{"tenant"}
	managementOnly := []environment.EnvironmentKind{"management"}
	onpremOnly := []environment.EnvironmentKind{"onprem"}
	all := append(append(tenantOnly, managementOnly...), onpremOnly...)

	str := &feature.Config{Type: feature.ConfigTypeString}
	intCfg := &feature.Config{Type: feature.ConfigTypeInt}
	boolCfg := &feature.Config{Type: feature.ConfigTypeBool}
	secret := &feature.Config{Type: feature.ConfigTypeString, Secret: true}
	strArr := &feature.Config{Type: feature.ConfigTypeStringArray}

	// Feature catalog. Each feature has 5–10 values mixing:
	//   - plain configs (some Required)
	//   - secret configs (Secret: true)
	//   - computed values that reference non-secret data
	//   - computed values that reference secret env values or secret configs
	//     (these are the *computed secrets* the env-config view masks)
	featureValues := map[string]feature.Values{
		"naiserator": {
			"replicas":      {DisplayName: "Replicas", Description: "Number of replicas", Required: true, Config: intCfg},
			"logLevel":      {DisplayName: "Log Level", Config: str},
			"apiKey":        {DisplayName: "API Key", Description: "External API key", Required: true, Config: secret},
			"clusterDomain": {DisplayName: "Cluster Domain", Description: "Derived from environment name", Computed: &feature.Computed{Template: `"{{ .Env.name }}.{{ .Tenant.Name }}.cloud.nais.io"`}},
			"projectRef":    {DisplayName: "GCP Project Ref", Computed: &feature.Computed{Template: `"projects/{{ .Env.project_id }}"`}},
			"imageTag":      {DisplayName: "Image Tag", Description: "Override image tag; falls back to a computed default", Config: str, Computed: &feature.Computed{Template: `"{{ .Env.name }}-latest"`}},
			"featureFlags":  {DisplayName: "Feature Flags", Description: "JSON blob of toggles", Config: str},
			"extraEnv":      {DisplayName: "Extra Env", Description: "Additional KEY=VALUE pairs", Config: strArr},
			"motd":          {DisplayName: "Message of the Day", Description: "Multi-line banner shown in the UI", Config: str},
			"allowedHosts":  {DisplayName: "Allowed Hosts", Description: "Required list; chart default is empty so warns until set", Required: true, Config: strArr},
			// Computed secret: reads the secret env value db_password.
			"dbDsn": {DisplayName: "DB DSN", Description: "Derived from secret env db_password", Computed: &feature.Computed{Template: `"postgres://naiserator:{{ .Env.db_password }}@db.{{ .Env.name }}.local/naiserator"`}},
		},
		"console": {
			"adminEmail":       {DisplayName: "Admin Email", Required: true, Config: str},
			"sessionSecret":    {DisplayName: "Session Secret", Config: secret},
			"debugMode":        {DisplayName: "Debug Mode", Config: boolCfg},
			"port":             {DisplayName: "Port", Config: intCfg},
			"oauthClientId":    {DisplayName: "OAuth Client ID", Config: str},
			"baseUrl":          {DisplayName: "Base URL", Computed: &feature.Computed{Template: `"https://console.{{ .Env.name }}.{{ .Tenant.Name }}.nais.io"`}},
			"oauthRedirectUri": {DisplayName: "OAuth Redirect URI", Computed: &feature.Computed{Template: `"https://console.{{ .Env.name }}.{{ .Tenant.Name }}.nais.io/oauth/callback"`}},
			// Computed secret: reads the secret env value slack_token.
			"slackWebhook": {DisplayName: "Slack Webhook URL", Description: "Derived from secret env slack_token", Computed: &feature.Computed{Template: `"https://hooks.slack.com/services/{{ .Env.slack_token }}"`}},
		},
		"unleash": {
			"instanceCount": {DisplayName: "Instance Count", Config: intCfg},
			"dbPassword":    {DisplayName: "Database Password", Required: true, Config: secret},
			"dbHost":        {DisplayName: "Database Host", Required: true, Config: str},
			"dbName":        {DisplayName: "Database Name", Config: str},
			"adminToken":    {DisplayName: "Admin Token", Config: secret},
			"metricsPath":   {DisplayName: "Metrics Path", Config: str},
			// Computed secret: reads secret config dbPassword.
			"dbUrl": {DisplayName: "Database URL", Description: "Derived from secret config dbPassword", Computed: &feature.Computed{Template: `"postgres://unleash:{{ .Configs.dbPassword }}@{{ .Configs.dbHost }}/{{ .Configs.dbName }}"`}},
		},
		"replicator": {
			"syncInterval":  {DisplayName: "Sync Interval", Description: "Seconds between syncs", Config: str},
			"maxRetries":    {DisplayName: "Max Retries", Config: intCfg},
			"concurrency":   {DisplayName: "Concurrency", Required: true, Config: intCfg},
			"logLevel":      {DisplayName: "Log Level", Config: str},
			"targetCluster": {DisplayName: "Target Cluster", Computed: &feature.Computed{Template: `"{{ .Env.name }}-replica"`}},
			// Computed secret: reads secret env api_key.
			"apiAuthHeader": {DisplayName: "API Auth Header", Description: "Derived from secret env api_key", Computed: &feature.Computed{Template: `"Bearer {{ .Env.api_key }}"`}},
		},
		"v13s": {
			"clusterName":   {DisplayName: "Cluster Name", Required: true, Config: str},
			"dryRun":        {DisplayName: "Dry Run", Config: boolCfg},
			"scanInterval":  {DisplayName: "Scan Interval", Config: intCfg},
			"imageRegistry": {DisplayName: "Image Registry", Required: true, Config: str},
			"dbPassword":    {DisplayName: "Database Password", Config: secret},
			"apiEndpoint":   {DisplayName: "API Endpoint", Computed: &feature.Computed{Template: `"https://v13s.{{ .Env.name }}.nais.io"`}},
			// Computed secret: reads secret config dbPassword.
			"dbUrl": {DisplayName: "Database URL", Description: "Derived from secret config dbPassword", Computed: &feature.Computed{Template: `"postgres://v13s:{{ .Configs.dbPassword }}@db.{{ .Env.name }}/v13s"`}},
		},
		"dependencytrack": {
			"apiUrl":          {DisplayName: "API URL", Required: true, Config: str},
			"apiToken":        {DisplayName: "API Token", Config: secret},
			"adminEmail":      {DisplayName: "Admin Email", Config: str},
			"pollInterval":    {DisplayName: "Poll Interval", Config: intCfg},
			"notificationUrl": {DisplayName: "Notification URL", Computed: &feature.Computed{Template: `"https://dt.{{ .Env.name }}.{{ .Tenant.Name }}.nais.io/notify"`}},
			// Computed secret: reads secret env slack_token.
			"slackAlertUrl": {DisplayName: "Slack Alert URL", Description: "Derived from secret env slack_token", Computed: &feature.Computed{Template: `"https://hooks.slack.com/services/{{ .Env.slack_token }}/alerts"`}},
		},
		"kyverno": {
			"webhookTimeout":    {DisplayName: "Webhook Timeout", Description: "Timeout in seconds", Config: intCfg},
			"replicaCount":      {DisplayName: "Replica Count", Required: true, Config: intCfg},
			"webhookSigningKey": {DisplayName: "Webhook Signing Key", Config: secret},
			"webhookURL":        {DisplayName: "Webhook URL", Computed: &feature.Computed{Template: `"https://hooks.{{ .Env.name }}.{{ .Tenant.Name }}.example.com/kyverno"`}},
			"envKind":           {DisplayName: "Environment Kind", Computed: &feature.Computed{Template: `"{{ .Env.kind }}"`}},
			// Computed secret: reads secret config webhookSigningKey.
			"signedWebhookURL": {DisplayName: "Signed Webhook URL", Description: "Derived from secret config webhookSigningKey", Computed: &feature.Computed{Template: `"https://hooks.{{ .Env.name }}.{{ .Tenant.Name }}.example.com/kyverno?sig={{ .Configs.webhookSigningKey }}"`}},
		},
		"aivenator": {
			"aivenToken":   {DisplayName: "Aiven Token", Required: true, Config: secret},
			"projectName":  {DisplayName: "Project Name", Required: true, Config: str},
			"region":       {DisplayName: "Region", Config: str},
			"adminEmail":   {DisplayName: "Admin Email", Config: str},
			"serviceUrl":   {DisplayName: "Service URL", Computed: &feature.Computed{Template: `"https://aiven.{{ .Env.name }}.nais.io"`}},
			"dashboardUrl": {DisplayName: "Dashboard URL", Computed: &feature.Computed{Template: `"https://aiven.{{ .Env.name }}.nais.io/dashboard"`}},
			// Computed secret: reads secret env api_key.
			"apiAuthHeader": {DisplayName: "API Auth Header", Description: "Derived from secret env api_key", Computed: &feature.Computed{Template: `"Bearer {{ .Env.api_key }}"`}},
		},
		"hookd": {
			"webhookUrl":    {DisplayName: "Webhook URL", Required: true, Config: str},
			"webhookSecret": {DisplayName: "Webhook Secret", Config: secret},
			"port":          {DisplayName: "Port", Config: intCfg},
			"githubAppId":   {DisplayName: "GitHub App ID", Required: true, Config: str},
			"slackChannel":  {DisplayName: "Slack Channel", Config: str},
			// Computed secret: reads secret config webhookSecret.
			"signedCallback": {DisplayName: "Signed Callback URL", Description: "Derived from secret config webhookSecret", Computed: &feature.Computed{Template: `"{{ .Configs.webhookUrl }}?sig={{ .Configs.webhookSecret }}"`}},
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

	featureDescriptions := map[string]string{
		"naiserator":      "Kubernetes operator for NAIS applications",
		"v13s":            "Vulnerability scanning and reporting",
		"console":         "Web console for NAIS platform management",
		"unleash":         "Feature toggle management service",
		"replicator":      "Cross-namespace secret and configmap replication",
		"dependencytrack": "Software composition analysis and dependency tracking",
		"kyverno":         "Kubernetes policy engine",
		"aivenator":       "Aiven service provisioning operator",
		"hookd":           "GitHub deployment webhook handler",
	}

	addAssignment := func(name, version string, target environment.Labels, kinds []environment.EnvironmentKind) {
		seeder.AddAssignmentWithValues(name, version, target, kinds, featureValues[name], featureDefaults[name], featureDescriptions[name])
	}

	naiseratorV := newVersion()
	naiseratorOldV := newVersion()
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
	kyvernoOldV := newVersion()

	// Seed older versions first — these will be automatically deactivated when
	// the current versions are deployed to the same target, exercising the
	// Seed older versions that get deactivated, exercising the
	// deactivation flow in local dev.
	addAssignment("naiserator", naiseratorOldV, environment.Labels{"kind": "tenant"}, tenantOnly)
	addAssignment("kyverno", kyvernoOldV, environment.Labels{}, all)

	addAssignment("naiserator", naiseratorV, environment.Labels{"kind": "tenant"}, tenantOnly)
	addAssignment("v13s", v13sV, environment.Labels{"kind": "management"}, managementOnly)
	addAssignment("console", consoleV, environment.Labels{"kind": "management"}, managementOnly)
	addAssignment("unleash", unleashV, environment.Labels{"kind": "management", "aiven": "enabled"}, managementOnly)
	addAssignment("replicator", replicatorV, environment.Labels{"kind": "tenant", "tenant": "test-partner", "name": "prod"}, tenantOnly)
	addAssignment("dependencytrack", dependencytrackV, environment.Labels{"kind": "tenant", "tenant": "test-partner", "name": "staging"}, tenantOnly)
	addAssignment("naiserator", naiseratorDevV, environment.Labels{"kind": "tenant", "tenant": "dev-nais", "name": "dev"}, tenantOnly)
	addAssignment("dependencytrack", dependencytrackDevV, environment.Labels{"kind": "tenant", "tenant": "dev-nais", "name": "dev"}, tenantOnly)
	addAssignment("replicator", replicatorDevV, environment.Labels{"kind": "tenant", "tenant": "dev-nais", "name": "dev"}, tenantOnly)
	addAssignment("unleash", unleashDevV, environment.Labels{"kind": "management", "tenant": "dev-nais"}, managementOnly)
	addAssignment("kyverno", kyvernoV, environment.Labels{}, all)

	ctx = auth.SetEmail(ctx, "tronghn@nais/fasit/123456789")
	if _, err := seeder.Seed(ctx); err != nil {
		log.With("err", err).Error("fatal")
		os.Exit(1)
	}

	ctx = auth.SetEmail(ctx, "setup_local_env")

	// Persist featureDefaults as configurations_global rows so that
	// Required-field validation passes for features that ship chart defaults
	// (the helm tab and deploy path both validate). In production these
	// would typically be set by an operator via the UI.
	for featureName, defaults := range featureDefaults {
		for key, val := range defaults {
			b, err := json.Marshal(val)
			if err != nil {
				log.With("err", err, "feature", featureName, "key", key).Error("marshal default")
				continue
			}
			// Skip empty values: they wouldn't satisfy required-field validation,
			// and seeding them as globals would mask the chart default in the UI,
			// preventing the required-but-unset warning from rendering.
			if isEmptyJSONValue(b) {
				continue
			}
			if _, err := feature.ConfigGlobalCreate(ctx, feature.NewConfiguration{
				Feature: featureName,
				Key:     key,
				Value:   json.RawMessage(b),
			}); err != nil {
				if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					log.With("err", err, "feature", featureName, "key", key).Error("seed global default")
				}
			}
		}
	}

	// Features are enabled by default (absence from disabled_features = enabled).
	// Disable one feature to get a DISABLED status row in the mix.
	if err := feature.FeatureDisable(ctx, envID("dev-nais", "dev"), "dependencytrack", "seeded as disabled for local dev"); err != nil {
		log.With("err", err).Error("disable dependencytrack in dev-nais/dev")
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
		if _, err := feature.ConfigEnvCreate(ctx, feature.NewConfiguration{
			EnvironmentID: &id,
			Feature:       o.feature,
			Key:           o.key,
			Value:         json.RawMessage(o.value),
		}); err != nil {
			log.With("err", err, "feature", o.feature, "key", o.key, "tenant", o.tenant, "env", o.env).Error("seed override")
		}
	}

	// Seed additional audit log entries to exercise the full range of actions
	// and make the audit tab useful during local development.
	{
		// Simulate different actors performing actions
		actors := []string{"johnny@nais.io", "kim@nais.io", "system:naisd", "setup_local_env"}

		// Enable a previously disabled feature (re-enable dependencytrack in a different env)
		devNaisDev := envID("dev-nais", "dev")
		_ = feature.FeatureEnable(ctx, devNaisDev, "dependencytrack")
		// Then disable it again with a different reason (generates disable+enable+disable sequence)
		_ = feature.FeatureDisable(ctx, devNaisDev, "dependencytrack", "re-disabled after testing: broken metrics endpoint")

		// Update environment labels (exercises SetLabels audit)
		ctx = auth.SetEmail(ctx, actors[1])
		testPartnerDev := envID("test-partner", "dev")
		_ = environment.SetLabels(ctx, testPartnerDev, environment.Labels{
			"monitoring": "enabled",
			"tier":       "development",
		})

		// Update a config value (exercises ActionUpdated for configs)
		ctx = auth.SetEmail(ctx, actors[0])
		confs, _ := feature.ConfigGet(ctx, "naiserator")
		for _, c := range confs {
			if c.Key == "logLevel" {
				_, _ = feature.ConfigUpdate(ctx, c.ID, feature.UpdateConfiguration{
					Value: json.RawMessage(`"warn"`),
				})
				break
			}
		}

		// Delete a config (exercises ActionDeleted for configs)
		ctx = auth.SetEmail(ctx, actors[1])
		confs, _ = feature.ConfigGet(ctx, "hookd")
		for _, c := range confs {
			if c.Key == "slackChannel" {
				_ = feature.ConfigDelete(ctx, c.ID)
				break
			}
		}

		// Delete an environment value
		ctx = auth.SetEmail(ctx, actors[2])
		testPartnerProd := envID("test-partner", "prod")
		_ = environment.DeleteEnvironmentValue(ctx, testPartnerProd, "updated_at")

		// Restore actor for the rest of the seeder
		ctx = auth.SetEmail(ctx, "setup_local_env")
	}

	// Set up pubsub topics and subscriptions.
	client, err := pubsub.NewClient(ctx, naisProjectID)
	if err != nil {
		log.With("err", err).Error("fatal")
		os.Exit(1)
	}

	topicRes, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/" + naisProjectID + "/topics/" + statusTopic,
	})
	if err != nil {
		log.With("err", err).Error("error")
	}
	_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:  "projects/" + naisProjectID + "/subscriptions/" + statusSubscription,
		Topic: topicRes.GetName(),
	})
	if err != nil {
		log.With("err", err).Error("error")
	}

	for tenant, envs := range envs {
		for env := range envs {
			topic := fmt.Sprintf("projects/%v/topics/naisd-%v-%v", naisProjectID, tenant, env)
			subscription := fmt.Sprintf("projects/%v/subscriptions/naisd-subscription", "local-"+tenant+"-"+env)

			_, err = client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topic})
			if err != nil {
				log.With("err", err).Error("error")
			}

			envClient, err := pubsub.NewClient(ctx, "local-"+tenant+"-"+env)
			if err != nil {
				log.With("err", err).Error("fatal")
				os.Exit(1)
			}
			_, err = envClient.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
				Name:  subscription,
				Topic: topic,
			})
			if err != nil {
				log.With("err", err).Error("error")
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
