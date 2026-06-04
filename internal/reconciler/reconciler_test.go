//go:build integration_test

package reconciler_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

var (
	provider = metricsdk.NewMeterProvider()
	meter    = provider.Meter("test-meter")
)

func TestListLatestFeatureAssignmentsFiltersInactive(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn := startPostgres(ctx, t)
	_ = container

	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seeder := featureassignmenttest.NewSeeder()
	featureassignment.ChartDownloader = seeder.ChartDownloader()

	loadContext, err := contextloader.NewLoaderFunc(pool, logger)
	if err != nil {
		t.Fatalf("create loader: %v", err)
	}
	ctx = loadContext(ctx)

	// Broad deployment v1, then override v2 for same target (deactivates v1)
	seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
	seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant"})
	ids, err := seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// v1 is inactive (deactivated by v2 create), v2 is active
	// Now deactivate v2 — simulates "remove override"
	if _, err := featureassignment.Deactivate(ctx, ids[1]); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// Query the reconciler uses
	querier := reconcilersql.New(pool)
	rows, err := querier.ListLatestFeatureAssignments(ctx)
	if err != nil {
		t.Fatalf("ListLatestFeatureAssignments: %v", err)
	}

	// After both are inactive, there should be NO deployments to reconcile
	if len(rows) != 0 {
		t.Errorf("ListLatestFeatureAssignments() returned %d rows, want 0 (all inactive)", len(rows))
		for _, row := range rows {
			t.Logf("  got: feature=%s version=%s", row.FeatureName, row.Version)
		}
	}
}

func TestListLatestFeatureAssignmentsPicksBroadAfterOverrideRemoved(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn := startPostgres(ctx, t)
	_ = container

	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seeder := featureassignmenttest.NewSeeder()
	featureassignment.ChartDownloader = seeder.ChartDownloader()

	loadContext, err := contextloader.NewLoaderFunc(pool, logger)
	if err != nil {
		t.Fatalf("create loader: %v", err)
	}
	ctx = loadContext(ctx)

	// Broad deployment v1 (targets all tenants)
	seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
	// Specific override v2 (targets tenant=dev-nais)
	seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant", "tenant": "dev-nais"})
	ids, err := seeder.Seed(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Both are active (different targets). Now deactivate the specific override.
	if _, err := featureassignment.Deactivate(ctx, ids[1]); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// The reconciler should still see v1 (broad, active)
	querier := reconcilersql.New(pool)
	rows, err := querier.ListLatestFeatureAssignments(ctx)
	if err != nil {
		t.Fatalf("ListLatestFeatureAssignments: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("ListLatestFeatureAssignments() returned %d rows, want 1", len(rows))
	}
	if rows[0].Version != "1.0.0" {
		t.Errorf("ListLatestFeatureAssignments()[0].Version = %q, want %q", rows[0].Version, "1.0.0")
	}
}

func startPostgres(ctx context.Context, t *testing.T) (*postgres.PostgresContainer, string) {
	t.Helper()
	container, err := postgres.Run(ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { testcontainers.CleanupContainer(t, container) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	// Run migrations
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect for migrations: %v", err)
	}
	pool.Close()

	return container, dsn
}
