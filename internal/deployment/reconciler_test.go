//go:build integration_test

package deployment_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
			loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
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

			_, err = mgr.seeder.Seed(ctx)
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

	mgr := setupTestMgr(ctx, t, container, dsn, logger)
	deployment.ChartDownloader = mgr.seeder.ChartDownloader()
	newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return mgr.publisher
	}
	loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
	if err != nil {
		t.Fatalf("failed to get setup context: %v", err)
	}
	ctx = loadContext(ctx)

	mgr.db.createTenantsAndEnvironments(ctx, envsToCreate)

	reconcilerCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	reconcilerCtx = loadContext(reconcilerCtx)

	mgr.seeder.AddDeployment("feature-pending", "1.0.0", environment.Labels{"aiven": "enabled"})
	_, err = mgr.seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seeding deployments: %v", err)
	}
	if err = deployment.GetManager(ctx).Reconcile(reconcilerCtx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	mgr.seeder.Reset()

	mgr.seeder.AddDeployment("feature-pending", "2.0.0", environment.Labels{})
	_, err = mgr.seeder.Seed(ctx)
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

func TestReconcileDisabledFeature(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("disabled feature is not deployed", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		if err != nil {
			t.Fatalf("failed to get setup context: %v", err)
		}
		ctx := loadContext(ctx)

		// Create two tenant environments.
		envsToCreate := map[string]environment.Labels{
			"tenant1:dev":  {"kind": "tenant"},
			"tenant1:prod": {"kind": "tenant"},
		}
		mgr.db.createTenantsAndEnvironments(ctx, envsToCreate)

		// Disable "clamav" in prod.
		var prodEnvID uuid.UUID
		row := mgr.db.pool.QueryRow(ctx, `SELECT e.id FROM environments e JOIN tenants t ON t.id = e.tenant_id WHERE t.name = 'tenant1' AND e.name = 'prod'`)
		if err := row.Scan(&prodEnvID); err != nil {
			t.Fatalf("get prod env id: %v", err)
		}
		_, err = mgr.db.pool.Exec(ctx, `INSERT INTO disabled_features (environment_id, feature) VALUES ($1, 'clamav')`, prodEnvID)
		if err != nil {
			t.Fatalf("insert disabled feature: %v", err)
		}

		// Create a deployment for clamav targeting kind=tenant (matches both dev and prod).
		mgr.seeder.AddDeployment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
		_, err = mgr.seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seeding deployments: %v", err)
		}

		// Reconcile.
		if err := deployment.GetManager(ctx).Reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		// Only dev should have received a deploy instruction.
		assert.Len(t, mgr.publisher.msg, 1)
		assert.Equal(t, "clamav", mgr.publisher.msg[0].Name)

		// Query deployment statuses — prod should show DISABLED.
		var deploymentID uuid.UUID
		row = mgr.db.pool.QueryRow(ctx, `SELECT id FROM deployments WHERE feature_name = 'clamav'`)
		if err := row.Scan(&deploymentID); err != nil {
			t.Fatalf("get deployment id: %v", err)
		}

		statuses, err := deployment.ListDeploymentStatuses(ctx, deploymentID)
		if err != nil {
			t.Fatalf("list deployment statuses: %v", err)
		}

		var devStatus, prodStatus *deployment.DeploymentStatus
		for _, s := range statuses {
			if s.EnvironmentID == prodEnvID {
				prodStatus = s
			} else {
				devStatus = s
			}
		}

		if prodStatus == nil {
			t.Fatal("expected a status entry for prod")
		}
		assert.Equal(t, deployment.DeploymentStatusStateDisabled, prodStatus.State)
		assert.Equal(t, "feature is disabled in this environment", prodStatus.Message)

		if devStatus == nil {
			t.Fatal("expected a status entry for dev")
		}
		// Status is CREATED because the test publisher doesn't simulate naisd
		// feedback; the important thing is that a deploy instruction was sent.
		assert.Equal(t, deployment.DeploymentStatusStateCreated, devStatus.State)
	})

	t.Run("re-enabling allows future deploys", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		if err != nil {
			t.Fatalf("failed to get setup context: %v", err)
		}
		ctx := loadContext(ctx)

		envsToCreate := map[string]environment.Labels{
			"tenant1:dev":  {"kind": "tenant"},
			"tenant1:prod": {"kind": "tenant"},
		}
		mgr.db.createTenantsAndEnvironments(ctx, envsToCreate)

		// Disable clamav in prod.
		var prodEnvID uuid.UUID
		row := mgr.db.pool.QueryRow(ctx, `SELECT e.id FROM environments e JOIN tenants t ON t.id = e.tenant_id WHERE t.name = 'tenant1' AND e.name = 'prod'`)
		if err := row.Scan(&prodEnvID); err != nil {
			t.Fatalf("get prod env id: %v", err)
		}
		_, err = mgr.db.pool.Exec(ctx, `INSERT INTO disabled_features (environment_id, feature) VALUES ($1, 'clamav')`, prodEnvID)
		if err != nil {
			t.Fatalf("insert disabled feature: %v", err)
		}

		// Deploy clamav v1.
		mgr.seeder.AddDeployment("clamav", "0.1.0", environment.Labels{"kind": "tenant"})
		_, err = mgr.seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seeding deployments: %v", err)
		}
		if err := deployment.GetManager(ctx).Reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		// Re-enable by removing the disabled row.
		_, err = mgr.db.pool.Exec(ctx, `DELETE FROM disabled_features WHERE environment_id = $1 AND feature = 'clamav'`, prodEnvID)
		if err != nil {
			t.Fatalf("delete disabled feature: %v", err)
		}

		// Deploy new version.
		mgr.seeder.Reset()
		mgr.publisher.msg = nil
		mgr.seeder.AddDeployment("clamav", "0.2.0", environment.Labels{"kind": "tenant"})
		_, err = mgr.seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seeding deployments: %v", err)
		}
		if err := deployment.GetManager(ctx).Reconcile(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		// Both environments should now have deploy instructions.
		assert.Len(t, mgr.publisher.msg, 2)

		var deploymentID uuid.UUID
		row = mgr.db.pool.QueryRow(ctx, `SELECT id FROM deployments WHERE feature_name = 'clamav' AND version = '0.2.0'`)
		if err := row.Scan(&deploymentID); err != nil {
			t.Fatalf("get deployment id: %v", err)
		}

		statuses, err := deployment.ListDeploymentStatuses(ctx, deploymentID)
		if err != nil {
			t.Fatalf("list deployment statuses: %v", err)
		}

		for _, s := range statuses {
			assert.Equal(t, deployment.DeploymentStatusStateCreated, s.State, "env %s should be CREATED (deploy sent)", s.EnvironmentID)
		}
	})
}

func TestReconcileGlobalDeployment(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	// Create environments of different kinds.
	envsToCreate := map[string]environment.Labels{
		"tenant1:dev":        {"kind": "tenant"},
		"tenant1:prod":       {"kind": "tenant"},
		"tenant1:management": {"kind": "management"},
	}

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	mgr := setupTestMgr(ctx, t, container, dsn, logger)
	deployment.ChartDownloader = mgr.seeder.ChartDownloader()

	newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return mgr.publisher
	}
	loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
	if err != nil {
		t.Fatalf("failed to get setup context: %v", err)
	}
	ctx = loadContext(ctx)

	mgr.db.createTenantsAndEnvironments(ctx, envsToCreate)

	// A global deployment has an empty target — should match ALL environments.
	mgr.seeder.AddDeployment("global-tool", "1.0.0", environment.Labels{})
	_, err = mgr.seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seeding deployments: %v", err)
	}

	if err := deployment.GetManager(ctx).Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// All 3 environments should receive deploy instructions.
	assert.Len(t, mgr.publisher.msg, 3)

	q := `
		SELECT
			t.name || ':' || e.name
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed' AND di.feature_name = 'global-tool'
		ORDER BY e.name
	`
	rows := mgr.db.runQuery(ctx, t, q)
	var deployed []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		deployed = append(deployed, s)
	}

	assert.Len(t, deployed, 3)
	assert.Contains(t, deployed, "tenant1:dev")
	assert.Contains(t, deployed, "tenant1:prod")
	assert.Contains(t, deployed, "tenant1:management")
}
