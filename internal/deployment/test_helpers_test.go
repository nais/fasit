//go:build integration_test

package deployment_test

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

var (
	provider = metricsdk.NewMeterProvider()
	meter    = provider.Meter("test-meter")
)

// Db is intentionally uppercase to avoid var clashes with other tests.
type Db struct {
	t    *testing.T
	pool *pgxpool.Pool
}

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
	env, err := environment.Create(ctx, &model.EnvironmentCreate{
		Name:     name,
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKind(labels["kind"]),
	})
	if err != nil {
		d.t.Fatalf("create environment: %v", err)
	}
	lbls := environment.Labels{}
	maps.Copy(lbls, labels)
	lbls["tenant"] = tenant.Name
	lbls["environment"] = env.Name
	if err := environment.SetLabels(ctx, env.ID, lbls); err != nil {
		d.t.Fatalf("set environment labels: %v", err)
	}
	if err := naisdstatus.Set(ctx, env.ID, &message.Health{ReportedAt: time.Now()}); err != nil {
		d.t.Fatalf("create health status: %v", err)
	}
}

func (d *Db) createTenantsAndEnvironments(ctx context.Context, tenantsAndEnvs map[string]environment.Labels) {
	d.t.Helper()
	tenants := make(map[string]*model.Tenant)
	for te, lbls := range tenantsAndEnvs {
		p := strings.Split(te, ":")
		tenantName, envName := p[0], p[1]
		if _, exists := tenants[tenantName]; !exists {
			tenant, err := environment.CreateTenant(ctx, &model.TenantCreate{Name: tenantName})
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
	container, err = postgres.Run(ctx,
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
	publisher *oldPublisher
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
	pub := &oldPublisher{}
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
	return Db{t: t, pool: pool}
}

// oldPublisher uses deployment.UpdateDeployInstructionStatus via context.
type oldPublisher struct {
	msg []message.DeployInstruction
}

func (p *oldPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)
	status := model.RolloutStatusDeployed
	if strings.HasSuffix(msg.Name, "-pending") {
		status = model.RolloutStatusPending
	}
	return deployment.UpdateDeployInstructionStatus(ctx, msg.ID, status)
}

func (p *oldPublisher) Stop() {}
