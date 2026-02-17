//go:build integration_test

package provider

import (
	"context"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestProvider(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresql(ctx, t)
	pool := newPool(ctx, t, container, dsn)
	log, _ := test.NewNullLogger()
	loadContext := func(ctx context.Context) context.Context {
		ctx = environment.Register(ctx, pool)
		ctx = audit.Register(ctx, pool, log)
		return ctx
	}

	c := startGrpcServer(t, loadContext, pool)

	t.Run("tenant operations", func(t *testing.T) {
		want, err := c.CreateTenant(ctx, &protogen.CreateTenantRequest{Name: "test-tenant"})
		if err != nil {
			t.Fatal(err)
		}

		got, err := c.GetTenant(ctx, &protogen.GetTenantRequest{Name: "test-tenant"})
		if err != nil {
			t.Fatal(err)
		}

		if got.GetId() != want.GetId() {
			t.Fatalf("Expected tenant id %s, got %s", want.GetId(), got.GetId())
		}
	})
}

// startGrpcServer initializes an in-memory gRPC server
func startGrpcServer(t *testing.T, loadContext contextloader.LoaderFunc, pool *pgxpool.Pool) protogen.ProviderClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	log, _ := test.NewNullLogger()
	repo := database.NewRepo(pool, log)
	grpcServer := NewGrpcServer(loadContext, repo)

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

	c := protogen.NewProviderClient(conn)

	t.Cleanup(func() {
		grpcServer.Stop()
		ioconvenience.CloseWithLog(conn, log)
	})
	return c
}

func startPostgresql(ctx context.Context, t *testing.T) (container *postgres.PostgresContainer, dsn string) {
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

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	logger, _ := test.NewNullLogger()
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if err = container.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}

	return container, dsn
}

func newPool(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string) *pgxpool.Pool {
	t.Helper()
	log, _ := test.NewNullLogger()
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

	return pool
}
