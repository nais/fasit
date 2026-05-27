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
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
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

	queryTracer, err := database.NewQueryMetricsTracer(meter)
	if err != nil {
		return fmt.Errorf("error creating query metrics tracer: %w", err)
	}

	pool, closers, err := database.NewConnPool(ctx, cfg.DBConnectionDSN, log, database.WithQueryTracer(queryTracer))
	if err != nil {
		return fmt.Errorf("error setting up database: %w", err)
	}
	defer ioconvenience.CloseWithLog(closers, log)

	if err := database.RegisterPoolMetrics(meter, pool); err != nil {
		return fmt.Errorf("error registering pool metrics: %w", err)
	}

	deploymentPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		p := message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
		p.SetMeter(meter)
		return p
	}

	loadContext, err := contextloader.NewLoaderFunc(pool, deploymentPublisher, meter, log)
	if err != nil {
		return fmt.Errorf("creating setup context: %w", err)
	}

	ctx = loadContext(ctx)
	go deployment.TimeoutDeployInstructions(ctx, log)

	go deployment.RunReconciler(ctx, 10*time.Minute)

	if cfg.UseNewReconciler {
		reconcilerPublisher := func(topicID string, log logrus.FieldLogger) reconciler.Publisher {
			p := message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
			p.SetMeter(meter)
			return p
		}
		rec, err := reconciler.New(reconcilersql.New(pool), reconcilerPublisher, meter, log.WithField("component", "reconciler"))
		if err != nil {
			return fmt.Errorf("creating reconciler: %w", err)
		}
		go rec.Run(ctx, 10*time.Minute)
		log.Info("new reconciler enabled")
	}

	statusMgr := message.NewSubscriber[message.Status](pubSubClient, cfg.GCPProjectID, cfg.StatusSubscriptionID, log)

	receiver := workers.NewReceiver(
		statusMgr,
		log,
		slackClient,
		cfg.SlackChannelFeatureAlerts,
		meter,
		deployment.GetManager(ctx),
	)
	go receiver.Run(ctx)

	go func() {
		if err := runGRPC(ctx, loadContext, cfg.GRPCBindAddress, log); err != nil {
			log.WithError(err).Fatal("running GRPC server")
		}
	}()

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	httpServer, err := newHTTPServer(serverCtx, loadContext, pool, cfg, meter, log)
	if err != nil {
		return fmt.Errorf("error creating http server: %w", err)
	}

	go func() {
		log.Printf("listening on http://%s/", cfg.HTTPBindAddress)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Fatal("running server")
		}
	}()

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

func runGRPC(ctx context.Context, loadContext contextloader.LoaderFunc, bindAddress string, log logrus.FieldLogger) error {
	log.Info("GRPC serving on port", bindAddress)
	lis, err := net.Listen("tcp", bindAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s := provider.NewGrpcServer(loadContext)

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

	prefixView := func(i metricsdk.Instrument) (metricsdk.Stream, bool) {
		s := metricsdk.Stream{Name: "fasit_" + i.Name}
		return s, true
	}

	return metricsdk.
		NewMeterProvider(
			metricsdk.WithReader(exporter),
			metricsdk.WithView(prefixView),
		).
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
