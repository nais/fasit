package deployment_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

// Intentional uppercase to avoid var clashes
type Db struct {
	repo database.Repo
	t    *testing.T
	pool *pgxpool.Pool
}

type featureInput struct {
	name, version string
	dependencies  []string
	target        environment.Labels
}

var (
	provider = metricsdk.NewMeterProvider()
	meter    = provider.Meter("test-meter")
)

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	envsToCreate := map[string]environment.Labels{
		"test-partner:dev":  {},
		"test-partner:prod": {"featuretoggle": "enabled"},
		"nav:dev":           {"aiven": "enabled"},
		"nav:management":    {"kind": "management"},
	}

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	fmt.Println("postgres container started: ", dsn)

	tt := []struct {
		name                string
		deploymentsToCreate []featureInput
		reconcileResults    [][]string
	}{
		{
			name: "install most specific and latest features",
			deploymentsToCreate: []featureInput{
				{name: "aivenator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "aivenator", version: "2.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "aivenator", version: "1.1.0", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
				{name: "aivenator", version: "1.1.1", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
				{name: "aivenator", version: "3.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "naiserator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "unleash", version: "1.0.0", target: environment.Labels{"featuretoggle": "enabled"}},
				{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
				{name: "v13s", version: "1.0.0", target: environment.Labels{"kind": "management"}},
			},
			reconcileResults: [][]string{
				{
					"nav:dev:aivenator:1.1.1",
					"nav:dev:naiserator:1.0.0",
					"nav:dev:unleash:2.0.0",
					"nav:management:v13s:1.0.0",
					"test-partner:dev:unleash:2.0.0",
					"test-partner:prod:unleash:2.0.0",
				},
			},
		},
		{
			name: "install features with dependencies",
			deploymentsToCreate: []featureInput{
				{
					name:    "monitoring",
					version: "v1",
					dependencies: []string{
						"monitoring-crds",
					},
					target: environment.Labels{"tenant": "nav"},
				},
				{
					name:    "monitoring-crds",
					version: "v1",
					target:  environment.Labels{"tenant": "nav"},
				},
			},
			reconcileResults: [][]string{
				{
					"nav:dev:monitoring-crds:v1",
					"nav:management:monitoring-crds:v1",
				},
				{
					"nav:dev:monitoring-crds:v1",
					"nav:management:monitoring-crds:v1",
					"nav:dev:monitoring:v1",
					"nav:management:monitoring:v1",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mgr := setupTestMgr(ctx, t, container, dsn, logger)
			deployment.ChartDownloader = mgr.seeder.ChartDownloader()

			newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
				return mgr.publisher
			}
			loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, nil, meter, logger)
			if err != nil {
				t.Fatalf("failed to get setup context: %v", err)
			}
			ctx = loadContext(ctx)

			reconcilerCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			reconcilerCtx = loadContext(reconcilerCtx)

			mgr.db.createTenantsAndEnvironments(ctx, envsToCreate)
			for _, input := range tc.deploymentsToCreate {
				mgr.seeder.AddDeployment(input.name, input.version, input.target, input.dependencies...)
			}

			err = mgr.seeder.Seed(ctx)
			if err != nil {
				t.Fatalf("seeding deployments: %v", err)
			}

			for _, result := range tc.reconcileResults {
				if err := deployment.GetManager(ctx).Reconcile(reconcilerCtx); err != nil {
					t.Fatalf("reconcile: %v", err)
				}

				q := `
				SELECT
				t.name || ':' || e.name || ':' || di.feature_name || ':' || di.feature_version
				FROM deploy_instructions di
				JOIN environments e ON e.id = di.environment_id
				JOIN tenants t ON t.id = e.tenant_id
				WHERE di.status = 'deployed'
			`

				rows := mgr.db.runQuery(ctx, t, q)

				var deployInstructions []string
				for rows.Next() {
					var instruction string
					_ = rows.Scan(&instruction)
					deployInstructions = append(deployInstructions, instruction)
				}

				assert.Len(t, deployInstructions, len(result))
				assert.Len(t, mgr.publisher.msg, len(result))

				for _, instruction := range result {
					found := false
					for _, msg := range mgr.publisher.msg {
						parts := strings.Split(instruction, ":")
						if fmt.Sprintf("%s:%s", msg.Name, msg.Version) == strings.Join(parts[2:], ":") {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("expected log message for instruction %q not found", instruction)
					}
				}
			}
		})
	}
}

func TestReconcileWhenPreviousIsInProgress(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	envsToCreate := map[string]environment.Labels{
		"nav:dev": {"aiven": "enabled"},
	}

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	fmt.Println("postgres container started: ", dsn)

	mgr := setupTestMgr(ctx, t, container, dsn, logger)
	deployment.ChartDownloader = mgr.seeder.ChartDownloader()
	newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return mgr.publisher
	}
	loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, nil, meter, logger)
	if err != nil {
		t.Fatalf("failed to get setup context: %v", err)
	}
	ctx = loadContext(ctx)

	mgr.db.createTenantsAndEnvironments(ctx, envsToCreate)

	reconcilerCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	reconcilerCtx = loadContext(reconcilerCtx)

	mgr.seeder.AddDeployment("feature-pending", "1.0.0", environment.Labels{"aiven": "enabled"})
	err = mgr.seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seeding deployments: %v", err)
	}
	if err = deployment.GetManager(ctx).Reconcile(reconcilerCtx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	mgr.seeder.Reset()

	mgr.seeder.AddDeployment("feature-pending", "2.0.0", environment.Labels{})
	err = mgr.seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seeding deployments: %v", err)
	}
	if err = deployment.GetManager(ctx).Reconcile(reconcilerCtx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	q := `
		SELECT
		COUNT(di.*) AS count
		FROM deploy_instructions di
		WHERE di.feature_name = 'feature-pending' AND di.feature_version = '2.0.0'
	`

	row := mgr.db.pool.QueryRow(ctx, q)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan deploy instructions: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected 0 instruction with status deployed, got %d", count)
	}
}

type publisher struct {
	msg  []message.DeployInstruction
	repo database.Repo
}

func (p *publisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)

	status := model.RolloutStatusDeployed
	if strings.HasSuffix(msg.Name, "-pending") {
		status = model.RolloutStatusPending
	}
	return p.repo.DeployInstructionUpdateStatus(ctx, msg.ID, status)
}

func (p *publisher) Stop() {}

func (d *Db) runQuery(ctx context.Context, t *testing.T, q string) pgx.Rows {
	r, err := d.pool.Query(ctx, q)
	if err != nil {
		t.Fatalf("run query: %v", err)
	}
	return r
}

func (d *Db) createEnv(ctx context.Context, tenant *model.Tenant, name string, labels environment.Labels) {
	d.t.Helper()

	if labels["kind"] == "" {
		labels["kind"] = "tenant"
	}
	env, err := d.repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     name,
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKind(labels["kind"]),
	})
	if err != nil {
		d.t.Fatalf("create environment: %v", err)
	}
	lbls := make([]*protogen.EnvironmentLabel, 0)
	for k, v := range labels {
		lbls = append(lbls, &protogen.EnvironmentLabel{
			Key:   k,
			Value: v,
		})
	}
	lbls = append(lbls, &protogen.EnvironmentLabel{
		Key:   "tenant",
		Value: tenant.Name,
	}, &protogen.EnvironmentLabel{
		Key:   "environment",
		Value: name,
	})
	err = d.repo.EnvironmentSetLabels(ctx, env.ID, lbls)
	if err != nil {
		d.t.Fatalf("set environment labels: %v", err)
	}

	err = naisdstatus.Set(ctx, env.ID, &message.Health{
		ReportedAt: time.Now(),
	})
	if err != nil {
		d.t.Fatalf("create health status: %v", err)
	}
}

// createTenantsAndEnvironments creates a set of tenants and environments.
func (d *Db) createTenantsAndEnvironments(ctx context.Context, tenantsAndEnvs map[string]environment.Labels) {
	d.t.Helper()

	tenants := make(map[string]*model.Tenant)
	for te, lbls := range tenantsAndEnvs {
		p := strings.Split(te, ":")
		tenantName, envName := p[0], p[1]

		_, exists := tenants[tenantName]
		if !exists {
			var err error
			tenant, err := environment.CreateTenant(ctx, &model.TenantCreate{
				Name: tenantName,
			})
			if err != nil {
				d.t.Fatalf("create tenant: %v", err)
			}

			tenants[tenantName] = tenant
		}

		d.createEnv(ctx, tenants[tenantName], envName, lbls)
	}
}

func startPostgresql(ctx context.Context, t *testing.T) (container *postgres.PostgresContainer, dsn string, err error) {
	t.Helper()

	container, err = postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	defer testcontainers.CleanupContainer(t, container)

	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	logger, _ := test.NewNullLogger()
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("Error connecting to database: %v", err)
	}
	pool.Close()

	if err = container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, dsn, nil
}

type TestMgr struct {
	t         *testing.T
	db        Db
	seeder    *deploymenttest.Seeder
	publisher *publisher
	log       logrus.FieldLogger
}

func setupTestMgr(
	ctx context.Context,
	t *testing.T,
	container *postgres.PostgresContainer,
	dsn string,
	log logrus.FieldLogger,
) *TestMgr {
	t.Helper()
	db := getDb(ctx, t, container, dsn, log)
	seeder := deploymenttest.NewSeeder()
	pub := &publisher{repo: db.repo}
	return &TestMgr{
		t:         t,
		db:        db,
		seeder:    seeder,
		publisher: pub,
		log:       log,
	}
}

func getDb(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log logrus.FieldLogger) Db {
	t.Helper()

	pool, _, err := database.NewConnPool(ctx, dsn, log)
	if err != nil {
		t.Fatalf("Error connecting to database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if err = container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	return Db{
		repo: database.NewRepo(pool, log),
		t:    t,
		pool: pool,
	}
}
