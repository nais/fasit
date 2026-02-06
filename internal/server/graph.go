package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/rs/cors"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/metric"
)

const slowQueryEndpoint = false

func SetupGraph(loadContext contextloader.LoaderFunc, resolver *graph.Resolver, meter metric.Meter) (http.Handler, error) {
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

	slowDownQuery := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slowQueryEndpoint {
				time.Sleep(2 * time.Second)
			}
			h.ServeHTTP(w, r)
		})
	}

	handler := slowDownQuery(corsMW.Handler(graphServer))

	return contextMiddleware(loadContext)(handler), nil
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
