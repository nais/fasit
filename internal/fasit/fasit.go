package fasit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/cluster"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/nais/fasit/internal/rollout"
	"github.com/nais/fasit/internal/slack"
	"github.com/nais/fasit/internal/workers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/rs/cors"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/attribute"
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

const slowQueryEndpoint = false

func Run(ctx context.Context) error {
	if err := loadEnvFile(); err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	cfg, err := newConfig(ctx, envconfig.OsLookuper())
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	log, err := newLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("error creating logger: %w", err)
	}

	log.Info("starting pub/sub client")
	pubSubClient, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		return fmt.Errorf("error creating pub/sub client: %w", err)
	}
	defer ioconvenience.CloseWithLog(pubSubClient, log)
	log.Info("-- successfully started pub/sub client")

	meter, err := newMetricsProvider()
	if err != nil {
		return fmt.Errorf("error creating metrics provider: %w", err)
	}

	slackClient := slack.New(cfg.SlackAPIToken)

	log.Info("starting database client")

	pool, closers, err := database.NewConnPool(ctx, cfg.DBConnectionDSN, log)
	if err != nil {
		return fmt.Errorf("error setting up database: %w", err)
	}
	defer ioconvenience.CloseWithLog(closers, log)

	repo := database.NewRepo(pool, log)
	go repo.TimeoutDeployInstructions(ctx)
	log.Info("-- successfully started database client")

	deployCreatePublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
	}

	deploymentMgr, err := deployment.NewManager(repo, deployCreatePublisher, meter, log)
	if err != nil {
		return fmt.Errorf("error creating deployment manager: %w", err)
	}
	go deploymentMgr.Run(ctx, 10*time.Minute)

	statusMgr := message.NewSubscriber[message.Status](pubSubClient, cfg.GCPProjectID, cfg.StatusSubscriptionID, log)

	receiver := workers.NewReceiver(
		statusMgr,
		repo,
		log,
		slackClient,
		cfg.SlackChannelFeatureAlerts,
		deploymentMgr,
	)
	go receiver.Run(ctx)

	notifierService := notifier.New(pool, log)
	go notifierService.Run(ctx)

	createPublisher := func(topicID string, log logrus.FieldLogger) workers.Publisher {
		return message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
	}
	reconciler, err := workers.NewReconciler(repo, createPublisher, notifierService, meter, log)
	if err != nil {
		return fmt.Errorf("error creating reconciler: %w", err)
	}

	go func() {
		defer log.Error("reconciler listener stopped")
		if err := reconciler.Listen(ctx); err != nil {
			log.WithError(err).Fatal("setting up reconciler listener")
		}
	}()
	go reconciler.Run(ctx, 10*time.Minute)

	costUpdater, err := workers.NewCostUpdater(ctx, repo, log)
	if err != nil {
		log.WithError(err).Error("setting up cost updater. You might need to run `gcloud auth --update-adc` if running locally")
	} else {
		go costUpdater.Run(ctx, 1*time.Hour)
	}

	googleClient, err := cluster.New(ctx)
	if err != nil {
		return fmt.Errorf("error creating google client: %w", err)
	}
	resolver := graph.NewResolver(ctx, repo, deploymentMgr, notifierService, createPublisher, googleClient, log)

	graphServer := newGraphServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))
	graphServer.Use(otelgqlgen.Middleware())
	metricsMW, err := graph.NewMetrics(meter)
	if err != nil {
		return fmt.Errorf("error creating metrics middleware: %w", err)
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
			return fmt.Errorf("INSECURE_SKIP_PROXY must be true when iap audience is not set")
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
		ghstatus, err := rollout.NewGHStatusReporter(log, repo, notifierService, os.Getenv("GH_PEM"))
		if err != nil {
			return fmt.Errorf("error creating github status reporter: %w", err)
		}
		go ghstatus.Run(ctx)
	}

	router := chi.NewMux()
	router.Handle("/", iapMW(playground.Handler("GraphQL playground", "/query")))
	router.Handle("/query", slowDownQuery(iapMW(corsMW.Handler(graphServer))))
	router.Handle("/metrics", promhttp.Handler())

	rout, err := rollout.New(ctx, repo)
	if err != nil {
		return fmt.Errorf("error creating rollout handler: %w", err)
	}
	rout.AllowAll = cfg.InsecureSkipTokenCheck
	router.Post("/github/rollout", rout.Rollout)

	deploy, err := deployment.NewHttpHandler(ctx, deploymentMgr, log)
	if err != nil {
		return fmt.Errorf("error creating deployment http handler: %w", err)
	}
	deploy.AllowAll = cfg.InsecureSkipTokenCheck
	router.Post("/github/deployment", deploy.CreateDeployment)
	router.Get("/github/deployment/{id}", deploy.GetDeployment)

	go func() {
		if err := runGRPC(ctx, cfg.GRPCBindAddress, repo, log); err != nil {
			panic(err)
		}
	}()

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

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

	if err := runClusterUpgrader(ctx, cfg.SlackClusterUpgradeChannel, log, googleClient, repo, meter, slackClient); err != nil {
		return fmt.Errorf("running cluster upgrader: %w", err)
	}

	go clustersMetrics(ctx, repo, meter, log)

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

	return nil
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

func newLogger(level string) (logrus.FieldLogger, error) {
	log := logrus.StandardLogger()
	log.SetFormatter(&logrus.JSONFormatter{})

	l, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("error parsing log level: %w", err)
	}
	log.SetLevel(l)
	return log, nil
}

func runGRPC(ctx context.Context, bindAddress string, repo database.Repo, log logrus.FieldLogger) error {
	log.Info("GRPC serving on port", bindAddress)
	lis, err := net.Listen("tcp", bindAddress)
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
	return metricsdk.
		NewMeterProvider(metricsdk.WithReader(exporter)).
		Meter("github.com/nais/fasit"), nil
}

func runClusterUpgrader(ctx context.Context, slackChannel string, log logrus.FieldLogger, clusterManager cluster.ClusterManager, repo database.Repo, meter metric.Meter, slack slack.SlackClient) error {
	s := workers.NewScheduler(log)
	clusterUpgrader := cluster.NewClusterUpgrader(repo, log, clusterManager, meter, slack, slackChannel)
	autoUpgrader := cluster.NewAutoUpgrader(repo, log, clusterManager, meter)

	s.Register("cluster-upgrader", clusterUpgrader, 30*time.Second)
	s.Register("auto-upgrader", autoUpgrader, 1*time.Hour)
	s.Start(ctx)

	return nil
}

func clustersMetrics(ctx context.Context, repo database.Repo, meter metric.Meter, log logrus.FieldLogger) {
	log = log.WithField("subsystem", "cluster-info")

	gauge, err := meter.Int64Gauge("cluster_info")
	if err != nil {
		return
	}

	for {
		tenants, err := repo.TenantsGet(ctx)
		if err != nil {
			log.WithError(err).Error("getting tenants")
			return
		}

		for _, tenant := range tenants {
			environments, err := repo.EnvironmentsGet(ctx, tenant.ID)
			if err != nil {
				log.WithError(err).Error("getting environments")
				return
			}

			for _, env := range environments {
				attr := []attribute.KeyValue{
					attribute.String("tenant", tenant.Name),
					attribute.String("environment", env.Name),
					attribute.String("kind", env.Kind.String()),
				}
				gauge.Record(ctx, 1, metric.WithAttributes(attr...))
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Minute):
		}
	}
}

// loadEnvFile will load a .env file if it exists. This is useful for local development.
func loadEnvFile() error {
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err := godotenv.Load(".env"); err != nil {
		return err
	}

	return nil
}
