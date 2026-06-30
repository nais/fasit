//go:build integration_test

package audit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func startPostgres(ctx context.Context, t *testing.T) (*postgres.PostgresContainer, string) {
	t.Helper()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"), postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, _, err := database.NewConnPool(ctx, dsn, discardLog())
	if err != nil {
		t.Fatalf("connect for migrations: %v", err)
	}
	pool.Close()
	if err := container.Snapshot(ctx); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return container, dsn
}

func searchTestContext(ctx context.Context, t *testing.T, dsn string) (context.Context, *pgxpool.Pool) {
	t.Helper()
	pool, _, err := database.NewConnPool(ctx, dsn, discardLog())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	ctx = Register(ctx, pool, discardLog())
	return ctx, pool
}

func exec(ctx context.Context, t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

// TestSearchRecentMatchesDisplay verifies that audit search matches the text a
// user sees in the table, not just the raw stored columns: the config value
// lives only in metadata (the original bug), and display labels diverge from
// stored enum values ("redeploy" shown as "redeployed", "configuration" as
// "config").
func TestSearchRecentMatchesDisplay(t *testing.T) {
	ctx := context.Background()
	_, dsn := startPostgres(ctx, t)
	ctx, pool := searchTestContext(ctx, t, dsn)

	tenantID := uuid.New()
	envID := uuid.New()
	exec(ctx, t, pool,
		fmt.Sprintf(`INSERT INTO tenants (id, name) VALUES ('%s', 'nav')`, tenantID),
		fmt.Sprintf(`INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'management', 'management')`, envID, tenantID),
		// Config create whose value ("true") exists only in metadata, in nav/management.
		fmt.Sprintf(`INSERT INTO audits (actor, description, object_type, object_id, action, environment_id, feature, metadata)
			VALUES ('sten@nais.io', '', 'configuration', 'nais-api-reconcilers/featureFlags.enableGrafanaAlerts', 'created', '%s', 'nais-api-reconcilers', '{"new":"true"}')`, envID),
		// Redeploy: stored action "redeploy" on an assignment, displayed as "redeployed".
		fmt.Sprintf(`INSERT INTO audits (actor, description, object_type, object_id, action, environment_id, feature)
			VALUES ('someone@nais.io', '', 'assignment', 'loki', 'redeploy', '%s', 'loki')`, envID),
	)

	tests := []struct {
		name      string
		terms     []string
		wantIDs   []string // object_ids expected in results
		wantEmpty bool
	}{
		{
			name:    "value from metadata is searchable with actor (the reported bug)",
			terms:   []string{"sten@nais.io", "true"},
			wantIDs: []string{"nais-api-reconcilers/featureFlags.enableGrafanaAlerts"},
		},
		{
			name:      "non-matching value excluded",
			terms:     []string{"sten@nais.io", "false"},
			wantEmpty: true,
		},
		{
			name:    "displayed action 'redeployed' matches stored 'redeploy'",
			terms:   []string{"redeployed", "loki"},
			wantIDs: []string{"loki"},
		},
		{
			name:    "displayed object type 'config' matches stored 'configuration'",
			terms:   []string{"config", "nav/management"},
			wantIDs: []string{"nais-api-reconcilers/featureFlags.enableGrafanaAlerts"},
		},
		{
			name:      "AND semantics: terms from different rows match nothing",
			terms:     []string{"sten@nais.io", "loki"},
			wantEmpty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SearchRecent(ctx, tc.terms, 200)
			if err != nil {
				t.Fatalf("SearchRecent(%v): %v", tc.terms, err)
			}
			if tc.wantEmpty {
				if len(got) != 0 {
					t.Fatalf("want no results, got %d", len(got))
				}
				return
			}
			gotIDs := make(map[string]bool, len(got))
			for _, e := range got {
				gotIDs[e.ObjectID] = true
			}
			for _, want := range tc.wantIDs {
				if !gotIDs[want] {
					t.Errorf("missing expected object_id %q in results %v", want, gotIDs)
				}
			}
			if len(got) != len(tc.wantIDs) {
				t.Errorf("got %d results, want %d (%v)", len(got), len(tc.wantIDs), gotIDs)
			}
		})
	}
}
