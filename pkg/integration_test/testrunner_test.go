package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/integration_test/testmanager"
	"github.com/nais/fasit/pkg/integration_test/testmanager/runner"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/sirupsen/logrus"
)

func TestRunner(t *testing.T) {
	mgr := testmanager.New(t, func(ctx context.Context, config testmanager.Config, state map[string]any) ([]testmanager.Runner, func(), error) {
		cleanups := []func(){}

		db, pool, cleanup, err := newDB(ctx, config, state)
		if err != nil {
			return nil, nil, err
		}
		cleanups = append(cleanups, cleanup)

		return []testmanager.Runner{
				newRestRunner(ctx, t, db),
				newGQLRunner(ctx, t, db),
				runner.NewSQLRunner(pool),
			}, func() {
				for _, cleanup := range cleanups {
					cleanup()
				}
			}, nil
	})

	ctx := context.Background()

	mgr.Run(ctx, os.DirFS("./testdata"))
}

func newRestRunner(ctx context.Context, t *testing.T, db rollout.Store) testmanager.Runner {
	router := chi.NewMux()
	// router.Handle("/query", iapMW(corsMW.Handler(srv)))

	rout, err := rollout.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	rout.AllowAll = true
	router.Post("/github/rollout", rout.Rollout)

	return runner.NewRestRunner(router)
}

func newGQLRunner(ctx context.Context, t *testing.T, db database.Repo) testmanager.Runner {
	log := logrus.New()
	log.Out = io.Discard

	resolver := &graph.Resolver{
		Repo: db,
		Log:  logrus.NewEntry(log),
		// HelmChartValues: helmChartValues,
	}

	newServer := func(es graphql.ExecutableSchema) *handler.Server {
		srv := handler.New(es)

		srv.AddTransport(transport.Websocket{
			KeepAlivePingInterval: 10 * time.Second,
			Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
				return true
			}},
		})
		srv.AddTransport(transport.GET{})
		srv.AddTransport(transport.POST{})

		return srv
	}

	srv := newServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))

	return runner.NewGQLRunner(srv)
}

func newDB(ctx context.Context, config testmanager.Config, state map[string]any) (database.Repo, *pgxpool.Pool, func(), error) {
	connStr, cleanup := dbtest.DockerSQLPool()

	pool, close, err := database.NewDB(ctx, connStr, false)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	log := logrus.New()
	log.Out = io.Discard

	db := database.New(pool, logrus.NewEntry(log))
	if err := database.Migrate("pgx", connStr, logrus.NewEntry(log)); err != nil {
		cleanup()
		close.Close()
		return nil, nil, nil, fmt.Errorf("Could not migrate: %w", err)
	}

	if err := seedTenantEnv(ctx, db, state, config); err != nil {
		cleanup()
		close.Close()
		return nil, nil, nil, fmt.Errorf("Could not seed: %w", err)
	}

	return db, pool, func() {
		cleanup()
		close.Close()
	}, nil
}
