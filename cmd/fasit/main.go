package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/nais/fasit"
	fdatabase "github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/message"
	fspanner "github.com/nais/fasit/pkg/spanner"
	"github.com/nais/fasit/pkg/spanner/migration"
	"github.com/nais/fasit/pkg/workers"
	"github.com/prometheus/client_golang/prometheus"
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
	flag.StringVar(&cfg.DBConnectionDSN, "database", "projects/nais-local-dev/instances/fasit/databases/fasit", "A valid database name has the form projects/PROJECT_ID/instances/INSTANCE_ID/databases/DATABASE_ID")
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

	db, err := setupDB(ctx, log, cfg.DBConnectionDSN)
	if err != nil {
		log.WithError(err).Fatal("unable to setup database")
	}
	defer db.Close()

	client, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up pubsub client")
	}

	var repo *fdatabase.Repo

	srepo := fspanner.NewRepo(db)

	// var dbDriver driver.Driver = pq.Driver{}
	// if !strings.Contains(cfg.DBConnectionDSN, "://") {
	// 	dbDriver = &postgres.Driver{}
	// }

	// repo, err := database.New(dbDriver, cfg.DBConnectionDSN, log.WithField("subsystem", "repo"))
	// if err != nil {
	// 	log.WithError(err).Fatal("setting up database")
	// }

	featureMgr, err := feature.New(fasit.FeaturesFS)
	if err != nil {
		log.WithError(err).Fatal("setting up features")
	}

	statusMgr := message.NewSubscriber[message.Status](client, cfg.GCPProjectID, cfg.StatusSubscriptionID)

	receiver := workers.NewReceiver(statusMgr, repo, log.WithField("subsystem", "status"))
	go receiver.Run(ctx)

	// reconciler := workers.NewReconciler(repo, featureMgr, client, cfg.GCPProjectID, log.WithField("subsystem", "reconciler"))
	// go reconciler.Run(ctx, 5*time.Minute)

	resolver := &graph.Resolver{
		Repo:     repo,
		SRepo:    srepo,
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

func setupDB(ctx context.Context, log *logrus.Logger, databaseName string) (*spanner.Client, error) {
	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	dataClient, err := spanner.NewClient(ctx, cfg.DBConnectionDSN)
	if err != nil {
		return nil, fmt.Errorf("unable to create spanner client: %w", err)
	}

	if err := migration.Migrate(adminClient, dataClient); err != nil {
		dataClient.Close()
		return nil, fmt.Errorf("unable to migrate database: %w", err)
	}

	return dataClient, nil
}
