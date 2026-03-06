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
	"github.com/joho/godotenv"
	"github.com/nais/fasit/internal/cluster"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/cost"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider"
	"github.com/nais/fasit/internal/rollout"
	"github.com/nais/fasit/internal/slack"
	"github.com/nais/fasit/internal/workers"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/sync/errgroup"
	// Supported database drivers.
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	_ "github.com/lib/pq"

	// Automatically set GOMAXPROCS to number of available CPUs. Might improve
	// performance in a containerized environment.
	_ "go.uber.org/automaxprocs"
)

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
	log.Info("-- successfully started database client")

	deploymentPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
	}
	rolloutPublisher := func(topicID string, log logrus.FieldLogger) rollout.Publisher {
		return message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
	}

	loadContext, err := contextloader.NewLoaderFunc(pool, deploymentPublisher, rolloutPublisher, meter, log)
	if err != nil {
		return fmt.Errorf("creating setup context: %w", err)
	}

	ctx = loadContext(ctx)
	go deployment.TimeoutDeployInstructions(ctx, log)

	go deployment.GetManager(ctx).Run(ctx, 10*time.Minute)

	statusMgr := message.NewSubscriber[message.Status](pubSubClient, cfg.GCPProjectID, cfg.StatusSubscriptionID, log)

	receiver := workers.NewReceiver(
		statusMgr,
		repo,
		log,
		slackClient,
		cfg.SlackChannelFeatureAlerts,
		deployment.GetManager(ctx),
	)
	go receiver.Run(ctx)

	notifierService := notifier.New(pool, log)
	go notifierService.Run(ctx)

	reconciler, err := rollout.NewReconciler(pool, rolloutPublisher, notifierService, meter, log)
	if err != nil {
		return fmt.Errorf("error creating reconciler: %w", err)
	}

	go func() {
		// TODO: this does not need to run in a goroutine as the Listen method is not blocking
		defer log.Info("reconciler listener started")
		if err := reconciler.Listen(ctx); err != nil {
			log.WithError(err).Fatal("setting up reconciler listener")
		}
	}()
	go reconciler.Run(ctx, 10*time.Minute)

	costUpdater, err := cost.NewCostUpdater(ctx, repo, log)
	if err != nil {
		log.WithError(err).Error("setting up cost updater. You might need to run `gcloud auth --update-adc` if running locally")
	} else {
		go costUpdater.Run(ctx, 1*time.Hour)
	}

	googleClient, err := cluster.New(ctx)
	if err != nil {
		return fmt.Errorf("error creating google client: %w", err)
	}
	defer ioconvenience.CloseWithLog(googleClient, log)

	if cfg.GitHubPEM != "" {
		log.Info("GitHub status reporter enabled")
		ghstatus, err := rollout.NewGHStatusReporter(log, repo, notifierService, cfg.GitHubPEM)
		if err != nil {
			return fmt.Errorf("error creating github status reporter: %w", err)
		}
		go ghstatus.Run(ctx)
	}

	go func() {
		if err := runGRPC(ctx, loadContext, cfg.GRPCBindAddress, repo, log); err != nil {
			log.WithError(err).Fatal("running GRPC server")
		}
	}()

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	httpServer, err := newHttpServer(serverCtx, loadContext, cfg, repo, notifierService, rolloutPublisher, googleClient, meter, log)
	if err != nil {
		return fmt.Errorf("error creating http server: %w", err)
	}

	go func() {
		log.Printf("connect to http://%s/ for GraphQL playground", cfg.HTTPBindAddress)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	if err := httpServer.Shutdown(timeoutCtx); err != nil {
		log.WithError(err).Error("shutting down server")
	}

	return nil
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

func runGRPC(ctx context.Context, loadContext contextloader.LoaderFunc, bindAddress string, repo database.Repo, log logrus.FieldLogger) error {
	log.Info("GRPC serving on port", bindAddress)
	lis, err := net.Listen("tcp", bindAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s := provider.NewGrpcServer(loadContext, repo)

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
