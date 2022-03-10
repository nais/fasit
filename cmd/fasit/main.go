package main

import (
	"context"
	"database/sql/driver"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	"github.com/lib/pq"
	"github.com/nais/fasit"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/workers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

var (
	cfg = DefaultConfig()

	promErrs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fasit",
		Name:      "errors",
	}, []string{"location"})
)

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.DBConnectionDSN, "db-connection-dsn", fmt.Sprintf("%v?sslmode=disable", getEnv("FASIT_DBCONN_STRING", "postgres://postgres:postgres@127.0.0.1:5432/fasit")), "database connection DSN")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.GCPProjectID, "project-id", "nais-local-dev", "Google project ID")
	flag.StringVar(&cfg.StatusSubscriptionID, "status-subscription-id", "fasit-subscription", "Pub/sub subscription for status")
}

func main() {
	flag.Parse()

	// ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	ctx := context.Background()
	// defer cancel()

	log := newLogger()

	client, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up pubsub client")
	}

	var dbDriver driver.Driver = pq.Driver{}
	if !strings.Contains(cfg.DBConnectionDSN, "://") {
		dbDriver = &postgres.Driver{}
	}

	repo, err := database.New(dbDriver, cfg.DBConnectionDSN, log.WithField("subsystem", "repo"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}

	featureMgr, err := feature.New(fasit.FeaturesFS)
	if err != nil {
		log.WithError(err).Fatal("setting up features")
	}

	statusMgr := message.NewSubscriber[message.Status](client, cfg.GCPProjectID, cfg.StatusSubscriptionID)

	receiver := workers.NewReceiver(statusMgr, repo, log.WithField("subsystem", "status"))
	go receiver.Run(ctx)

	reconciler := workers.NewReconciler(repo, featureMgr, client, cfg.GCPProjectID, log.WithField("subsystem", "reconciler"))
	go reconciler.Run(ctx, 5*time.Minute)

	resolver := &graph.Resolver{
		Repo:     repo,
		Features: featureMgr,
	}
	srv := handler.NewDefaultServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))

	corsMW := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})
	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", corsMW.Handler(srv))

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
