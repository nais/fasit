package fasit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/slack"
	"github.com/sethvargo/go-envconfig"
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

// Version is set at build time via ldflags.
var Version = "dev"

func Run(ctx context.Context) error {
	if err := loadEnvFile(); err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	cfg, err := newConfig(ctx, envconfig.OsLookuper())
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	log := newLogger(cfg.LogLevel)

	log.Info("starting pub/sub client")
	pubSubClient, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		return fmt.Errorf("error creating pub/sub client: %w", err)
	}
	defer ioconvenience.CloseWithLog(pubSubClient, log)
	log.Info("successfully started pub/sub client")

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

	assignmentPublisher := func(topicID string, log *slog.Logger) reconciler.Publisher {
		p := message.NewPublisher[message.DeployInstruction](pubSubClient, cfg.GCPProjectID, topicID, log)
		p.SetMeter(meter)
		return p
	}

	loadContext, err := contextloader.NewLoaderFunc(pool, log)
	if err != nil {
		return fmt.Errorf("creating setup context: %w", err)
	}

	ctx = loadContext(ctx)

	rec, err := reconciler.New(pool, meter, log.With("component", "reconciler"))
	if err != nil {
		return fmt.Errorf("creating reconciler: %w", err)
	}
	ctx = reconciler.WithContext(ctx, rec)

	go rec.TimeoutDeployInstructions(ctx, log)

	dispatcher, err := reconciler.NewPubSubDispatcher(pool, assignmentPublisher, meter, log)
	if err != nil {
		return err
	}

	go rec.Run(ctx, 1*time.Minute, dispatcher)

	statusMgr := message.NewSubscriber[message.Status](pubSubClient, cfg.GCPProjectID, cfg.StatusSubscriptionID, log)

	receiver := reconciler.NewReceiver(
		pool,
		statusMgr,
		log,
		slackClient,
		cfg.SlackChannelFeatureAlerts,
		meter,
	)
	go receiver.Run(ctx)

	go func() {
		if err := runGRPC(ctx, loadContext, cfg.GRPCBindAddress, log); err != nil {
			log.With("err", err).Error("running GRPC server")
		}
	}()

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	httpServer, err := newHTTPServer(serverCtx, loadContext, pool, cfg, meter, log)
	if err != nil {
		return fmt.Errorf("error creating http server: %w", err)
	}

	go func() {
		log.With("addr", "http://"+cfg.HTTPBindAddress+"/").Info("listening")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.With("err", err).Error("running server")
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
		log.With("err", err).Error("shutting down server")
	}

	return nil
}

func newLogger(level string) *slog.Logger {
	lvl := new(slog.LevelVar)
	switch level {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "warn", "warning":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

func runGRPC(ctx context.Context, loadContext contextloader.LoaderFunc, bindAddress string, log *slog.Logger) error {
	log.With("addr", bindAddress).Info("GRPC serving")
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
