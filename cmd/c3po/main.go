package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/nais/c3po"
	"github.com/nais/c3po/pkg/database"
	"github.com/nais/c3po/pkg/feature"
	"github.com/nais/c3po/pkg/graph"
	"github.com/nais/c3po/pkg/graph/graphgen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	cfg = DefaultConfig()

	promErrs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nada_backend",
		Name:      "errors",
	}, []string{"location"})
)

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.DBConnectionDSN, "db-connection-dsn", fmt.Sprintf("%v?sslmode=disable", getEnv("NAIS_DATABASE_C3PO_BACKEND_C3PO_URL", "postgres://postgres:postgres@127.0.0.1:5432/c3po")), "database connection DSN")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
}

func main() {
	flag.Parse()

	// ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// defer cancel()

	log := newLogger()
	repo, err := database.New(cfg.DBConnectionDSN, log.WithField("subsystem", "repo"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}

	featureMgr, err := feature.New(c3po.FeaturesFS)
	if err != nil {
		log.WithError(err).Fatal("setting up features")
	}

	resolver := &graph.Resolver{
		Repo:     repo,
		Features: featureMgr,
	}
	srv := handler.NewDefaultServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	log.Printf("connect to http://%s/ for GraphQL playground", cfg.BindAddress)
	log.Fatal(http.ListenAndServe(cfg.BindAddress, nil))
}

func newLogger() *logrus.Logger {
	log := logrus.StandardLogger()
	log.SetFormatter(&logrus.JSONFormatter{})

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(l)
	return log
}

func getEnv(key, fallback string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return fallback
}
