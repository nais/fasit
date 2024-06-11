package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/pkg/auth"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/notifier"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/provider"
	"github.com/nais/fasit/pkg/provider/protogen"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/nais/fasit/pkg/slack"
	"github.com/nais/fasit/pkg/upgrader"
	"github.com/nais/fasit/pkg/workers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	// Supported database drivers.
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	_ "github.com/lib/pq"

	// Automatically set GOMAXPROCS to number of available CPUs. Might improve
	// performance in a containerized environment.
	_ "go.uber.org/automaxprocs"
)

var cfg = DefaultConfig()

const slowQueryEndpoint = false

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.GRPCBindAddress, "grpc-bind-address", cfg.GRPCBindAddress, "Bind address")
	flag.StringVar(&cfg.DBConnectionDSN, "db-connection-dsn", getEnv("FASIT_DBCONN_STRING", "postgres://postgres:postgres@127.0.0.1:5432/fasit?sslmode=disable"), "database connection DSN")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.GCPProjectID, "project-id", "nais-local-dev", "Google project ID")
	flag.StringVar(&cfg.StatusSubscriptionID, "status-subscription-id", "fasit-subscription", "Pub/sub subscription for status")
	flag.StringVar(&cfg.IAPAudience, "iap-audience", "", "IAP audience string")
	flag.BoolVar(&cfg.InsecureSkipProxy, "insecure-skip-proxy", false, "Insecure, but allows the server to not require iap")
	flag.BoolVar(&cfg.InsecureSkipTokenCheck, "insecure-skip-token-check", false, "Insecure, but allows the server ignore token check")
	flag.StringVar(&cfg.SlackClusterUpgradeChannel, "slackChannel", os.Getenv("SLACK_CLUSTER_UPGRADE_CHANNEL"), "Slack channel to send message to")
	flag.StringVar(&cfg.SlackAPIToken, "slackAPIToken", os.Getenv("SLACK_API_TOKEN"), "Slack API token")
}

func newServer(es graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)
	srv.AddTransport(transport.SSE{}) // Support subscriptions
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New(1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
	})

	return srv
}

func main() {
	flag.Parse()

	ctx := context.Background()
	// defer cancel()

	log := newLogger()

	log.Info("starting pubsub client")
	pubsubClient, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up pubsub client")
	}
	log.Info("-- successfully started pubsub client")

	meter, err := newMetricsProvider()
	if err != nil {
		log.WithError(err).Fatal("setting up metrics provider")
	}

	slackClient := slack.New(cfg.SlackAPIToken)

	log.Info("starting database client")
	dbDriver := "pgx"
	if !strings.Contains(cfg.DBConnectionDSN, "://") {
		dbDriver = "cloudsql-postgres"
	}

	extraDSN := ""
	if runtime.NumCPU() < 5 {
		extraDSN = " pool_max_conns=5"
	}

	db, closers, err := database.NewDB(ctx, cfg.DBConnectionDSN+extraDSN, dbDriver != "pgx")
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}
	defer func() {
		if err := closers.Close(); err != nil {
			log.WithError(err).Errorf("closing database: %v", err)
		}
	}()

	log.Infof("migrating database with connection %s", cfg.DBConnectionDSN)

	if err := database.Migrate(dbDriver, cfg.DBConnectionDSN, log.WithField("subsystem", "migrate")); err != nil {
		log.WithError(err).Fatal("migrating database")
	}

	repo := database.New(db, log.WithField("subsystem", "repo"))
	log.Info("-- successfully started database client")

	statusMgr := message.NewSubscriber[message.Status](pubsubClient, cfg.GCPProjectID, cfg.StatusSubscriptionID)

	receiver := workers.NewReceiver(statusMgr, repo, log.WithField("subsystem", "status"))
	go receiver.Run(ctx)

	notifierService := notifier.New(db, log.WithField("subsystem", "notifier"))
	go notifierService.Run(ctx)

	createPublisher := func(topicID string, log *logrus.Entry) workers.Publisher {
		return message.NewPublisher[message.DeployInstruction](pubsubClient, cfg.GCPProjectID, topicID, log)
	}
	reconciler, err := workers.NewReconciler(repo, createPublisher, notifierService, meter, log.WithField("subsystem", "reconciler"))
	if err != nil {
		log.WithError(err).Fatal("setting up reconciler")
	}

	go func() {
		defer log.Error("reconciler listener stopped")
		if err := reconciler.Listen(ctx); err != nil {
			log.WithError(err).Fatal("setting up reconciler listener")
		}
	}()
	go reconciler.Run(ctx, 10*time.Minute)

	costUpdater, err := workers.NewCostUpdater(ctx, repo, log.WithField("subsystem", "cost_updater"))
	if err != nil {
		log.WithError(err).Error("setting up cost updater. You might need to run `gcloud auth --update-adc` if running locally")
	} else {
		go costUpdater.Run(ctx, 1*time.Hour)
	}

	googleClient, err := upgrader.New(ctx)
	if err != nil {
		log.WithError(err).Fatal("setting up google client")
	}
	resolver := graph.NewResolver(ctx, repo, notifierService, createPublisher, googleClient, log.WithField("subsystem", "graphql"))

	srv := newServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))
	srv.Use(otelgqlgen.Middleware())
	metricsMW, err := graph.NewMetrics(meter)
	if err != nil {
		log.WithError(err).Fatal("setting up metrics middleware")
	}
	srv.Use(metricsMW)

	corsMW := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})

	// Add the IAP validation middleware.
	// If the IAP audience is not set, we stop the server with a fatal error
	// unless the insecure-skip-proxy flag is set.
	iapMW := auth.ValidateJWTFromComputeEngine(cfg.IAPAudience)
	if cfg.IAPAudience == "" {
		if !cfg.InsecureSkipProxy {
			log.Fatal("IAP audience must be set")
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

	if os.Getenv("GH_PEM") != "" {
		log.Info("GitHub status reporter enabled")
		ghstatus, err := rollout.NewGHStatusReporter(log.WithField("subsystem", "gh_status"), repo, notifierService, os.Getenv("GH_PEM"))
		if err != nil {
			log.WithError(err).Fatal("setting up gh status reporter")
		}
		go ghstatus.Run(ctx)
	}

	router := chi.NewMux()
	router.Handle("/", iapMW(playground.Handler("GraphQL playground", "/query")))
	router.Handle("/query", slowDownQuery(iapMW(corsMW.Handler(srv))))
	router.Handle("/metrics", promhttp.Handler())

	rout, err := rollout.New(ctx, repo)
	if err != nil {
		log.WithError(err).Fatal("setting up rollout")
	}
	rout.AllowAll = cfg.InsecureSkipTokenCheck
	router.Post("/github/rollout", rout.Rollout)

	go func() {
		if err := runGRPC(ctx, repo, log); err != nil {
			panic(err)
		}
	}()

	serverCtx, serverCancel := context.WithCancel(ctx)
	server := &http.Server{
		Addr:              cfg.BindAddress,
		Handler:           router,
		ReadHeaderTimeout: 2 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return serverCtx
		},
	}

	go func() {
		log.Printf("connect to http://%s/ for GraphQL playground", cfg.BindAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("running server")
		}
	}()

	if err := runClusterUpgrader(ctx, log, googleClient, repo, meter, slackClient); err != nil {
		log.Fatal(err)
	}

	if err := runAutoUpgrader(ctx, log, googleClient, repo); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()
	<-ctx.Done()
	serverCancel()
	log.Info("Shutting down")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(timeoutCtx); err != nil {
		log.WithError(err).Error("shutting down server")
	}
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

func runGRPC(ctx context.Context, repo database.Repo, log logrus.FieldLogger) error {
	log.Info("GRPC serving on port", cfg.GRPCBindAddress)
	lis, err := net.Listen("tcp", cfg.GRPCBindAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{}
	s := grpc.NewServer(opts...)

	protogen.RegisterProviderServer(s, provider.NewServer(repo))

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.Serve(lis) })
	g.Go(func() error {
		<-ctx.Done()
		s.GracefulStop()
		return nil
	})

	return g.Wait()
}

func newMetricsProvider() (metric.Meter, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	provider := metricsdk.NewMeterProvider(metricsdk.WithReader(exporter))
	return provider.Meter("github.com/nais/fasit"), nil
}

func runClusterUpgrader(ctx context.Context, log *logrus.Logger, googleClient upgrader.Upgrader, repo database.Repo, meter metric.Meter, slack *slack.Slack) error {
	s := workers.NewScheduler(log.WithField("subsystem", "scheduler"))
	clusterUpgrader := upgrader.NewClusterUpgrader(repo, log, googleClient, meter, slack, cfg.SlackClusterUpgradeChannel)
	s.Register("cluster-upgrader", clusterUpgrader, 30*time.Second)
	s.Start(ctx)

	log.Info("Done")
	return nil
}

func runAutoUpgrader(ctx context.Context, log *logrus.Logger, googleClient upgrader.Upgrader, repo database.Repo) error {
	s := workers.NewScheduler(log.WithField("subsystem", "scheduler"))
	autoUpgrader := upgrader.NewAutoUpgrader(repo, log, googleClient)
	s.Register("auto-upgrader", autoUpgrader, 1*time.Hour)
	s.Start(ctx)

	log.Info("Done")
	return nil
}
