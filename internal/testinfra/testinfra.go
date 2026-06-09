//go:build integration_test

// Package testinfra provides shared integration test infrastructure.
// It starts a PostgreSQL container, runs migrations, and provides
// per-test pool/context helpers with snapshot/restore isolation.
package testinfra

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// DB holds a running postgres container and its DSN.
type DB struct {
	Container *postgres.PostgresContainer
	DSN       string
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Start launches a PostgreSQL container, connects, runs migrations, and
// takes a snapshot. Call once per TestMain or top-level test.
func Start(ctx context.Context, t *testing.T) *DB {
	t.Helper()

	container, err := postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	// Run migrations by connecting once.
	pool, _, err := database.NewConnPool(ctx, dsn, discardLogger())
	if err != nil {
		t.Fatalf("connect for migrations: %v", err)
	}
	pool.Close()

	if err := container.Snapshot(ctx); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	return &DB{Container: container, DSN: dsn}
}

// Pool returns a fresh connection pool for one test and registers cleanup
// that closes the pool and restores the DB snapshot.
func (db *DB) Pool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _, err := database.NewConnPool(ctx, db.DSN, discardLogger())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if err := db.Container.Restore(ctx); err != nil {
			t.Fatalf("restore snapshot: %v", err)
		}
	})
	return pool
}

// Context returns a context with common registrations (audit, environment,
// deployment, naisdstatus) wired to the given pool. Callers that need
// additional registrations (e.g. feature.Register) should add them after.
func Context(ctx context.Context, pool *pgxpool.Pool) context.Context {
	ctx = audit.Register(ctx, pool, discardLogger())
	ctx = environment.Register(ctx, pool)
	ctx = featureassignment.Register(ctx, pool)
	ctx = naisdstatus.Register(ctx, pool)
	return ctx
}

// Exec runs SQL statements against the pool. Useful for test fixtures.
func Exec(ctx context.Context, t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}
