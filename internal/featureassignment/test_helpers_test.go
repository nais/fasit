//go:build integration_test

package featureassignment_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"

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

func (d *Db) createEnv(ctx context.Context, tenant *environment.Tenant, name string, labels environment.Labels) {
	d.t.Helper()
	if labels["kind"] == "" {
		labels["kind"] = "tenant"
	}
	env, err := environment.Create(ctx, &environment.EnvironmentCreate{
		Name:     name,
		TenantID: tenant.ID,
		Kind:     environment.EnvironmentKind(labels["kind"]),
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
	tenants := make(map[string]*environment.Tenant)
	for te, lbls := range tenantsAndEnvs {
		p := strings.Split(te, ":")
		tenantName, envName := p[0], p[1]
		if _, exists := tenants[tenantName]; !exists {
			tenant, err := environment.CreateTenant(ctx, &environment.TenantCreate{Name: tenantName})
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
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
	t      *testing.T
	db     Db
	seeder *featureassignmenttest.Seeder
	log    *slog.Logger
}

func setupTestMgr(
	ctx context.Context,
	t *testing.T,
	container *postgres.PostgresContainer,
	dsn string,
	log *slog.Logger,
) *TestMgr {
	t.Helper()
	db := getDb(ctx, t, container, dsn, log)
	seeder := featureassignmenttest.NewSeeder()
	return &TestMgr{
		t:      t,
		db:     db,
		seeder: seeder,
		log:    log,
	}
}

func getDb(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log *slog.Logger) Db {
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
