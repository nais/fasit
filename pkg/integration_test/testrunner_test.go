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
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/nais/fasit/pkg/workers"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

const (
	tenantName        = "tenant23"
	envManagementName = "management"
	envTenantName     = "testing"
	envTenantNonCI    = "nonci"
)

func TestRunner(t *testing.T) {
	mgr := testmanager.New(t, func(ctx context.Context, config testmanager.Config, state map[string]any) ([]testmanager.Runner, func(), []testmanager.Option, error) {
		ctx, done := context.WithCancel(ctx)
		cleanups := []func(){done}

		opts := []testmanager.Option{}

		db, pool, cleanup, err := newDB(ctx, config, state)
		if err != nil {
			return nil, nil, opts, err
		}
		cleanups = append(cleanups, cleanup)

		naisdRunner, close, err := newNaisd(ctx, config, db)
		if err != nil {
			return nil, nil, opts, err
		}

		if naisdRunner != nil {
			cleanups = append(cleanups, close)
			tes, err := db.TenantEnvironments(ctx)
			if err != nil {
				return nil, nil, opts, err
			}

			for _, te := range tes {
				db.HealthStatusCreateOrUpdate(ctx, te.Environment.ID, &message.Health{
					ReportedAt: time.Now(),
				})
			}
		}

		if v, _ := config.Bool("reconcile"); v {
			log := logrus.New()
			// log.Out = os.Stdout
			// log.Level = logrus.DebugLevel
			log.Out = io.Discard
			cp := func(projectID, topicID string, log *logrus.Entry) workers.Publisher {
				p, ok := naisdRunner.reconcilerPublishers[topicID]
				if !ok {
					t.Fatalf("no publisher for topic %q", topicID)
				}
				return p
			}
			reconciler, err := workers.NewReconciler(db, cp, "", metric.NewNoopMeter(), logrus.NewEntry(log))
			if err != nil {
				return nil, nil, opts, err
			}
			opts = append(opts, testmanager.WithBeforeHook(func(ctx context.Context) {
				if err := reconciler.Reconcile(ctx); err != nil {
					t.Fatal(err)
				}
				time.Sleep(100 * time.Millisecond)
			}))
		}

		return []testmanager.Runner{
				newRestRunner(ctx, t, db),
				newGQLRunner(ctx, t, db),
				runner.NewSQLRunner(pool),
				naisdRunner,
			}, func() {
				for _, cleanup := range cleanups {
					cleanup()
				}
			}, opts, nil
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
