//go:build integration_test

package environmentmanagement

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environmentmanagement/protogen"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestCreateTenant(t *testing.T) {
	ctx := context.Background()
	container := startPostgresContainer(ctx, t)

	t.Run("creates a tenant", func(t *testing.T) {
		c, pool := newClientWithPool(ctx, t, container)

		resp, err := c.CreateTenant(ctx, &protogen.CreateTenantRequest{Name: "acme"})
		if err != nil {
			t.Fatalf("CreateTenant: %v", err)
		}
		if resp.GetTenant().GetName() != "acme" {
			t.Errorf("name = %q, want %q", resp.GetTenant().GetName(), "acme")
		}
		if resp.GetTenant().GetId() == "" {
			t.Error("expected a non-empty tenant id")
		}

		assertAudit(t, latestAudit(ctx, t, pool), audit.ActionCreated, audit.ObjectTypeTenant, resp.GetTenant().GetId())
	})

	t.Run("rejects a too-short name", func(t *testing.T) {
		c, pool := newClientWithPool(ctx, t, container)

		_, err := c.CreateTenant(ctx, &protogen.CreateTenantRequest{Name: "a"})
		requireCode(t, err, codes.InvalidArgument)

		assertNoAudit(ctx, t, pool)
	})
}

func TestGetTenant(t *testing.T) {
	ctx := context.Background()
	container := startPostgresContainer(ctx, t)

	t.Run("returns a created tenant by name", func(t *testing.T) {
		c := newClient(ctx, t, container)
		want := createTenant(ctx, t, c, "acme")

		got, err := c.GetTenant(ctx, &protogen.GetTenantRequest{Name: "acme"})
		if err != nil {
			t.Fatalf("GetTenant: %v", err)
		}
		if got.GetTenant().GetId() != want.GetId() {
			t.Errorf("id = %q, want %q", got.GetTenant().GetId(), want.GetId())
		}
		if got.GetTenant().GetName() != "acme" {
			t.Errorf("name = %q, want %q", got.GetTenant().GetName(), "acme")
		}
	})

	t.Run("returns not found for unknown tenant", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.GetTenant(ctx, &protogen.GetTenantRequest{Name: "does-not-exist"})
		requireCode(t, err, codes.NotFound)
	})
}

func TestCreateEnvironment(t *testing.T) {
	ctx := context.Background()
	container := startPostgresContainer(ctx, t)

	t.Run("creates an environment", func(t *testing.T) {
		c, pool := newClientWithPool(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")

		resp, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
			TenantId: tenant.GetId(),
			Name:     "dev",
			Kind:     protogen.EnvironmentKind_TENANT,
			Labels:   []*protogen.EnvironmentLabel{{Key: "team", Value: "platform"}},
		})
		if err != nil {
			t.Fatalf("CreateEnvironment: %v", err)
		}
		if resp.GetEnvironment().GetName() != "dev" {
			t.Errorf("name = %q, want %q", resp.GetEnvironment().GetName(), "dev")
		}
		if resp.GetEnvironment().GetTenantId() != tenant.GetId() {
			t.Errorf("tenant id = %q, want %q", resp.GetEnvironment().GetTenantId(), tenant.GetId())
		}

		got := latestAudit(ctx, t, pool)
		assertAudit(t, got, audit.ActionCreated, audit.ObjectTypeEnvironment, resp.GetEnvironment().GetId())
		if got.EnvironmentID == nil {
			t.Error("expected EnvironmentID to be set")
		}
	})

	t.Run("rejects a too-short name", func(t *testing.T) {
		c := newClient(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")

		_, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
			TenantId: tenant.GetId(),
			Name:     "x",
			Kind:     protogen.EnvironmentKind_TENANT,
		})
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("rejects an invalid tenant id", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
			TenantId: "not-a-uuid",
			Name:     "dev",
			Kind:     protogen.EnvironmentKind_TENANT,
		})
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("returns not found for unknown tenant", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
			TenantId: uuid.NewString(),
			Name:     "dev",
			Kind:     protogen.EnvironmentKind_TENANT,
		})
		requireCode(t, err, codes.NotFound)
	})

	t.Run("rejects an unknown kind", func(t *testing.T) {
		c := newClient(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")

		_, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
			TenantId: tenant.GetId(),
			Name:     "dev",
			Kind:     protogen.EnvironmentKind_UNKNOWN,
		})
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("accepts all known kinds", func(t *testing.T) {
		kinds := map[string]protogen.EnvironmentKind{
			"management": protogen.EnvironmentKind_MANAGEMENT,
			"tenant":     protogen.EnvironmentKind_TENANT,
			"onprem":     protogen.EnvironmentKind_ONPREM,
		}
		for name, kind := range kinds {
			t.Run(name, func(t *testing.T) {
				c := newClient(ctx, t, container)
				tenant := createTenant(ctx, t, c, "acme")

				if _, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
					TenantId: tenant.GetId(),
					Name:     name,
					Kind:     kind,
				}); err != nil {
					t.Fatalf("CreateEnvironment kind %s: %v", name, err)
				}
			})
		}
	})
}

func TestGetEnvironment(t *testing.T) {
	ctx := context.Background()
	container := startPostgresContainer(ctx, t)

	t.Run("returns an environment with labels", func(t *testing.T) {
		c := newClient(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")
		createEnvironment(ctx, t, c, tenant.GetId(), "dev", &protogen.EnvironmentLabel{Key: "team", Value: "platform"})

		resp, err := c.GetEnvironment(ctx, &protogen.GetEnvironmentRequest{
			TenantId: tenant.GetId(),
			Name:     "dev",
		})
		if err != nil {
			t.Fatalf("GetEnvironment: %v", err)
		}
		if resp.GetEnvironment().GetName() != "dev" {
			t.Errorf("name = %q, want %q", resp.GetEnvironment().GetName(), "dev")
		}
		labels := resp.GetEnvironment().GetLabels()
		if len(labels) != 1 || labels[0].GetKey() != "team" || labels[0].GetValue() != "platform" {
			t.Errorf("labels = %v, want one team=platform label", labels)
		}
	})

	t.Run("rejects an invalid tenant id", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.GetEnvironment(ctx, &protogen.GetEnvironmentRequest{
			TenantId: "not-a-uuid",
			Name:     "dev",
		})
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("returns not found for unknown environment", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.GetEnvironment(ctx, &protogen.GetEnvironmentRequest{
			TenantId: uuid.NewString(),
			Name:     "nope",
		})
		requireCode(t, err, codes.NotFound)
	})
}

func TestUpdateEnvironment(t *testing.T) {
	ctx := context.Background()
	container := startPostgresContainer(ctx, t)

	t.Run("updates labels", func(t *testing.T) {
		c, pool := newClientWithPool(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")
		env := createEnvironment(ctx, t, c, tenant.GetId(), "dev", &protogen.EnvironmentLabel{Key: "team", Value: "platform"})

		resp, err := c.UpdateEnvironment(ctx, &protogen.UpdateEnvironmentRequest{
			EnvironmentId: env.GetId(),
			Labels:        []*protogen.EnvironmentLabel{{Key: "team", Value: "infra"}},
		})
		if err != nil {
			t.Fatalf("UpdateEnvironment: %v", err)
		}
		labels := resp.GetEnvironment().GetLabels()
		if len(labels) != 1 || labels[0].GetKey() != "team" || labels[0].GetValue() != "infra" {
			t.Errorf("labels = %v, want one team=infra label", labels)
		}

		assertAudit(t, latestAudit(ctx, t, pool), audit.ActionUpdated, audit.ObjectTypeEnvironment, env.GetId())
	})

	t.Run("rejects an invalid environment id", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.UpdateEnvironment(ctx, &protogen.UpdateEnvironmentRequest{
			EnvironmentId: "not-a-uuid",
		})
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("returns not found for unknown environment", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.UpdateEnvironment(ctx, &protogen.UpdateEnvironmentRequest{
			EnvironmentId: uuid.NewString(),
		})
		requireCode(t, err, codes.NotFound)
	})
}

func TestEnvironmentValues(t *testing.T) {
	ctx := context.Background()
	container := startPostgresContainer(ctx, t)

	t.Run("set, get, overwrite and delete a value", func(t *testing.T) {
		c, pool := newClientWithPool(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")
		env := createEnvironment(ctx, t, c, tenant.GetId(), "dev")

		setResp, err := c.SetEnvironmentValue(ctx, &protogen.SetEnvironmentValueRequest{
			EnvironmentId: env.GetId(),
			Key:           "db_url",
			Value:         []byte(`"postgres://localhost"`),
			Secret:        true,
		})
		if err != nil {
			t.Fatalf("SetEnvironmentValue: %v", err)
		}
		if !setResp.GetSuccess() {
			t.Error("expected success")
		}

		assertAudit(t, latestAudit(ctx, t, pool), audit.ActionUpdated, audit.ObjectTypeEnvironmentValue, "db_url")

		got, err := c.GetEnvironmentValue(ctx, &protogen.GetEnvironmentValueRequest{
			EnvironmentId: env.GetId(),
			Key:           "db_url",
		})
		if err != nil {
			t.Fatalf("GetEnvironmentValue: %v", err)
		}
		if string(got.GetEnvironmentValue().GetValue()) != `"postgres://localhost"` {
			t.Errorf("value = %s, want %q", got.GetEnvironmentValue().GetValue(), `"postgres://localhost"`)
		}
		if !got.GetEnvironmentValue().GetSecret() {
			t.Error("expected secret = true")
		}
		if got.GetEnvironmentValue().GetTenantName() != "acme" {
			t.Errorf("tenant name = %q, want %q", got.GetEnvironmentValue().GetTenantName(), "acme")
		}
		if got.GetEnvironmentValue().GetEnvironmentName() != "dev" {
			t.Errorf("environment name = %q, want %q", got.GetEnvironmentValue().GetEnvironmentName(), "dev")
		}

		if _, err := c.SetEnvironmentValue(ctx, &protogen.SetEnvironmentValueRequest{
			EnvironmentId: env.GetId(),
			Key:           "db_url",
			Value:         []byte(`"postgres://other"`),
		}); err != nil {
			t.Fatalf("SetEnvironmentValue (overwrite): %v", err)
		}

		got, err = c.GetEnvironmentValue(ctx, &protogen.GetEnvironmentValueRequest{
			EnvironmentId: env.GetId(),
			Key:           "db_url",
		})
		if err != nil {
			t.Fatalf("GetEnvironmentValue after overwrite: %v", err)
		}
		if string(got.GetEnvironmentValue().GetValue()) != `"postgres://other"` {
			t.Errorf("value after overwrite = %s, want %q", got.GetEnvironmentValue().GetValue(), `"postgres://other"`)
		}
		if got.GetEnvironmentValue().GetSecret() {
			t.Error("expected secret = false after overwrite")
		}

		delResp, err := c.DeleteEnvironmentValue(ctx, &protogen.DeleteEnvironmentValueRequest{
			EnvironmentId: env.GetId(),
			Key:           "db_url",
		})
		if err != nil {
			t.Fatalf("DeleteEnvironmentValue: %v", err)
		}
		if !delResp.GetSuccess() {
			t.Error("expected delete success")
		}

		assertAudit(t, latestAudit(ctx, t, pool), audit.ActionDeleted, audit.ObjectTypeEnvironmentValue, "db_url")

		if _, err := c.GetEnvironmentValue(ctx, &protogen.GetEnvironmentValueRequest{
			EnvironmentId: env.GetId(),
			Key:           "db_url",
		}); err == nil {
			t.Fatal("expected error getting deleted value")
		}
	})

	t.Run("lists values for a key across environments", func(t *testing.T) {
		c := newClient(ctx, t, container)
		tenant := createTenant(ctx, t, c, "acme")
		dev := createEnvironment(ctx, t, c, tenant.GetId(), "dev")
		prod := createEnvironment(ctx, t, c, tenant.GetId(), "prod")

		for _, env := range []string{dev.GetId(), prod.GetId()} {
			if _, err := c.SetEnvironmentValue(ctx, &protogen.SetEnvironmentValueRequest{
				EnvironmentId: env,
				Key:           "shared",
				Value:         []byte(`"x"`),
			}); err != nil {
				t.Fatalf("SetEnvironmentValue: %v", err)
			}
		}

		resp, err := c.ListEnvironmentValues(ctx, &protogen.ListEnvironmentValuesRequest{Key: "shared"})
		if err != nil {
			t.Fatalf("ListEnvironmentValues: %v", err)
		}
		if len(resp.GetValues()) != 2 {
			t.Fatalf("got %d values, want 2", len(resp.GetValues()))
		}
		for _, v := range resp.GetValues() {
			if v.GetTenantName() != "acme" {
				t.Errorf("tenant name = %q, want %q", v.GetTenantName(), "acme")
			}
		}
	})

	t.Run("rejects an invalid environment id", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.SetEnvironmentValue(ctx, &protogen.SetEnvironmentValueRequest{
			EnvironmentId: "not-a-uuid",
			Key:           "k",
			Value:         []byte(`"v"`),
		})
		requireCode(t, err, codes.InvalidArgument)

		_, err = c.GetEnvironmentValue(ctx, &protogen.GetEnvironmentValueRequest{
			EnvironmentId: "not-a-uuid",
			Key:           "k",
		})
		requireCode(t, err, codes.InvalidArgument)

		_, err = c.DeleteEnvironmentValue(ctx, &protogen.DeleteEnvironmentValueRequest{
			EnvironmentId: "not-a-uuid",
			Key:           "k",
		})
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("returns not found for missing value", func(t *testing.T) {
		c := newClient(ctx, t, container)

		_, err := c.GetEnvironmentValue(ctx, &protogen.GetEnvironmentValueRequest{
			EnvironmentId: uuid.NewString(),
			Key:           "missing",
		})
		requireCode(t, err, codes.NotFound)
	})
}

// createTenant creates a tenant via the gRPC API and returns it, failing the test on error.
func createTenant(ctx context.Context, t *testing.T, c protogen.FasitClient, name string) *protogen.Tenant {
	t.Helper()
	resp, err := c.CreateTenant(ctx, &protogen.CreateTenantRequest{Name: name})
	if err != nil {
		t.Fatalf("createTenant(%q): %v", name, err)
	}
	return resp.GetTenant()
}

// createEnvironment creates a tenant-kind environment via the gRPC API and returns it, failing the test on error.
func createEnvironment(ctx context.Context, t *testing.T, c protogen.FasitClient, tenantID, name string, labels ...*protogen.EnvironmentLabel) *protogen.Environment {
	t.Helper()
	resp, err := c.CreateEnvironment(ctx, &protogen.CreateEnvironmentRequest{
		TenantId: tenantID,
		Name:     name,
		Kind:     protogen.EnvironmentKind_TENANT,
		Labels:   labels,
	})
	if err != nil {
		t.Fatalf("createEnvironment(%q): %v", name, err)
	}
	return resp.GetEnvironment()
}

// requireCode fails the test unless err is a gRPC status error with the given code.
func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	if got := status.Code(err); got != want {
		t.Fatalf("error code = %s, want %s (err: %v)", got, want, err)
	}
}

// latestAudit returns the most recent audit entry, failing the test if none exist.
func latestAudit(ctx context.Context, t *testing.T, pool *pgxpool.Pool) *audit.Entry {
	t.Helper()
	entries, err := audit.ListRecent(auditContext(ctx, pool), 1)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry, got none")
	}
	return entries[0]
}

// assertAudit checks that the entry was recorded by the provider with the expected action, object type and id.
func assertAudit(t *testing.T, e *audit.Entry, action audit.Action, objectType audit.ObjectType, objectID string) {
	t.Helper()
	if e.Actor != "system:provider" {
		t.Errorf("actor = %q, want %q", e.Actor, "system:provider")
	}
	if e.Action != action {
		t.Errorf("action = %q, want %q", e.Action, action)
	}
	if e.ObjectType != objectType {
		t.Errorf("object type = %q, want %q", e.ObjectType, objectType)
	}
	if e.ObjectID != objectID {
		t.Errorf("object id = %q, want %q", e.ObjectID, objectID)
	}
}

// assertNoAudit fails the test if any audit entries have been recorded.
func assertNoAudit(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	entries, err := audit.ListRecent(auditContext(ctx, pool), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d audit entries, want 0", len(entries))
	}
}

// newClient creates an isolated gRPC client backed by a freshly restored database. Each (sub-)test should call this to
// get its own client so tests do not share state.
func newClient(ctx context.Context, t *testing.T, container *postgres.PostgresContainer) protogen.FasitClient {
	t.Helper()
	c, _ := newClientWithPool(ctx, t, container)
	return c
}

// newClientWithPool is like newClient but also returns the underlying pool so tests can inspect persisted state, such
// as audit entries.
func newClientWithPool(ctx context.Context, t *testing.T, container *postgres.PostgresContainer) (protogen.FasitClient, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(ctx, t, container)
	return startGrpcServer(t, pool), pool
}

// startGrpcServer initializes an in-memory gRPC server
func startGrpcServer(t *testing.T, pool *pgxpool.Pool) protogen.FasitClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	loaderFunc := func(ctx context.Context) context.Context {
		return audit.Register(ctx, pool, discardLogger())
	}
	grpcServer := NewGrpcServer(pool, loaderFunc, discardLogger())
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			panic(err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		grpcServer.Stop()
		ioconvenience.CloseWithLog(conn, discardLogger())
	})

	return protogen.NewFasitClient(conn)
}

// startPostgresContainer starts a PostgreSQL container for testing and returns the container instance. All migrations
// are run, and a snapshot of the initialized database is taken so individual tests can quickly reset it via newPool.
func startPostgresContainer(ctx context.Context, t *testing.T) *postgres.PostgresContainer {
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
	defer testcontainers.CleanupContainer(t, container)

	if err != nil {
		t.Fatal(err)
	}

	dsn := connectionString(ctx, t, container)
	t.Logf("Started PostgreSQL container with DSN: %q", dsn)
	pool, _, err := database.NewConnPool(ctx, dsn, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if err := container.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}

	return container
}

// newPool creates a new pgxpool.Pool connected to the given container. The pool is closed and the database is restored
// to its snapshot when the (sub-)test finishes, isolating each test from the others.
func newPool(ctx context.Context, t *testing.T, container *postgres.PostgresContainer) *pgxpool.Pool {
	t.Helper()
	pool, _, err := database.NewConnPool(ctx, connectionString(ctx, t, container), discardLogger())
	if err != nil {
		t.Fatalf("Error connecting to database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if err = container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	return pool
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func connectionString(ctx context.Context, t *testing.T, container *postgres.PostgresContainer) string {
	t.Helper()
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Error getting connection string: %v", err)
	}
	return dsn
}

func auditContext(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return audit.Register(ctx, pool, discardLogger())
}
