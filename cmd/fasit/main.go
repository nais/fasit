package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/nais/fasit"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/status"
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
	flag.StringVar(&cfg.DBConnectionDSN, "db-connection-dsn", fmt.Sprintf("%v?sslmode=disable", getEnv("NAIS_DATABASE_fasit_BACKEND_fasit_URL", "postgres://postgres:postgres@127.0.0.1:5432/fasit")), "database connection DSN")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.GCPProjectID, "project-id", "banankake", "Google project ID")
	flag.StringVar(&cfg.StatusTopicID, "topic-id", "status", "Pub/sub topic for status")
}

func main() {
	flag.Parse()

	//ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	ctx := context.Background()
	//defer cancel()

	log := newLogger()
	repo, err := database.New(cfg.DBConnectionDSN, log.WithField("subsystem", "repo"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}

	featureMgr, err := feature.New(fasit.FeaturesFS)
	if err != nil {
		log.WithError(err).Fatal("setting up features")
	}

	mgr, err := status.New(ctx, cfg.GCPProjectID, cfg.StatusTopicID)
	if err != nil {
		log.WithError(err).Fatal("new status manager")
	}
	receiver := status.NewReceiver(mgr, "fasit", repo, log.WithField("subsystem", "status"))
	go receiver.Run(ctx)

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
