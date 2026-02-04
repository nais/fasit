package fasit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/cluster"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/rollout"
	"github.com/nais/fasit/internal/workers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/metric"
)

func newHttpServer(
	ctx context.Context,
	cfg *Config,
	repo database.Repo,
	deploymentManager *deployment.Manager,
	notifier *notifier.Notifier,
	publisher workers.NewPublisher,
	clusterClient *cluster.Client,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*http.Server, error) {
	resolver := graph.NewResolver(ctx, repo, deploymentManager, notifier, publisher, clusterClient, log)

	graphServer := newGraphServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))
	graphServer.Use(otelgqlgen.Middleware())
	metricsMW, err := graph.NewMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics middleware: %w", err)
	}
	graphServer.Use(metricsMW)

	corsMW := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})

	// Add the IAP validation middleware.
	// If the IAP audience is not set, we stop the server with a fatal error
	// unless the INSECURE_SKIP_PROXY env var is true.
	iapMW := auth.ValidateJWTFromComputeEngine(cfg.IAPAudience)
	if cfg.IAPAudience == "" {
		if !cfg.InsecureSkipProxy {
			return nil, fmt.Errorf("INSECURE_SKIP_PROXY must be true when iap audience is not set")
		}

		iapMW = auth.InsecureValidateMW
	}

	slowDownQuery := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slowQueryEndpoint {
				time.Sleep(2 * time.Second)
			}
			h.ServeHTTP(w, r)
		})
	}

	router := chi.NewMux()
	router.Handle("/", iapMW(playground.Handler("GraphQL playground", "/query")))
	router.Handle("/query", slowDownQuery(iapMW(corsMW.Handler(graphServer))))
	router.Handle("/metrics", promhttp.Handler())

	rout, err := rollout.New(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error creating rollout handler: %w", err)
	}
	rout.AllowAll = cfg.InsecureSkipTokenCheck
	router.Post("/github/rollout", rout.Rollout)

	deploy, err := deployment.NewHttpHandler(ctx, deploymentManager, log)
	if err != nil {
		return nil, fmt.Errorf("error creating deployment http handler: %w", err)
	}
	deploy.AllowAll = cfg.InsecureSkipTokenCheck
	router.Post("/github/deployment", deploy.CreateDeployment)
	router.Get("/github/deployment/{id}", deploy.GetDeployment)

	return &http.Server{
		Addr:              cfg.HTTPBindAddress,
		Handler:           router,
		ReadHeaderTimeout: 2 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}, nil
}

func newGraphServer(es graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)
	srv.AddTransport(transport.SSE{}) // Support subscriptions
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	return srv
}
