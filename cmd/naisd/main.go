package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/nais/fasit/cmd/naisd/local"
	"github.com/nais/fasit/internal/helm"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisd"
	"github.com/nais/fasit/internal/workers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	naisStatusTopic = "status"
)

var cfg = DefaultConfig()

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.EnvProjectID, "env-project-id", "local-test-dev", "Google project ID")
	flag.StringVar(&cfg.NaisProjectID, "nais-project-id", "nais-local-dev", "Nais project ID")
	flag.StringVar(&cfg.TenantName, "tenant-name", "test", "tenant name")
	flag.StringVar(&cfg.Env, "env", "dev", "cluster environment")
	flag.StringVar(&cfg.DeploySubscription, "deploy-subscription", "naisd-subscription", "name of subscription with deploy instructions from Fasit")
	flag.BoolVar(&cfg.Production, "production", false, "When in production, actually run helm install")
	flag.BoolVar(&cfg.Management, "management", false, "if naisd is running in a management cluster")
	flag.BoolVar(&cfg.MockFailing, "mock-failing", false, "fail execution of helm command when running locally")
}

func main() {
	flag.Parse()
	log := newLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if flag.Arg(0) == "upgrade" {
		upgrade(ctx, log)
		return
	}

	if envLvl := os.Getenv("LOG_LEVEL"); envLvl != "" {
		// Override log level from environment
		var lvl slog.LevelVar
		switch envLvl {
		case "debug":
			lvl.Set(slog.LevelDebug)
		case "warn", "warning":
			lvl.Set(slog.LevelWarn)
		case "error":
			lvl.Set(slog.LevelError)
		default:
			lvl.Set(slog.LevelInfo)
		}
		log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: &lvl}))
	}

	if err := run(ctx, log); err != nil {
		log.With("err", err).Error("fatal")
		os.Exit(1)
	}

	log.Info("Run cancelled, exiting. If we've started an upgrade, we'll keep running until it's done.")
	select {
	case <-ctx.Done():
		log.Info("Shutting down")
	case <-time.After(30 * time.Minute):
		log.Error("Shutdown timed out")
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	meter, err := newNaisdMetricsProvider()
	if err != nil {
		return err
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		srv := &http.Server{Addr: cfg.BindAddress, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.With("err", err).Error("metrics server")
		}
	}()

	receiver, helmClient, statusPublisher := sharedDependencies(ctx, log, meter)

	s := workers.NewScheduler(log.With("subsystem", "scheduler"))
	helmListReporter := naisd.NewStatusReporter(cfg.TenantName, cfg.Env, helmClient, statusPublisher)
	healthReporter := naisd.NewHealthReporter(cfg.TenantName, cfg.Env, statusPublisher)
	s.Register("helm-list", helmListReporter, 15*time.Minute)
	s.Register("health", healthReporter, 1*time.Minute)
	s.Start(ctx)

	receiver.RepublishHelmList = helmListReporter.Trigger

	log.Info("Receiver started")
	receiver.Run(ctx)
	return nil
}

func ensureAnnotation(ctx context.Context, client kubernetes.Interface, id string) error {
	ns, err := client.CoreV1().Namespaces().Get(ctx, "nais-system", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if metav1.HasAnnotation(ns.ObjectMeta, "cnrm.cloud.google.com/project-id") {
		return nil
	}

	metav1.SetMetaDataAnnotation(&ns.ObjectMeta, "cnrm.cloud.google.com/project-id", id)
	_, err = client.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})

	return err
}

func newLogger() *slog.Logger {
	lvl := new(slog.LevelVar)
	switch cfg.LogLevel {
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

func upgrade(ctx context.Context, log *slog.Logger) {
	log.Info("Upgrading naisd")
	meter, err := newNaisdMetricsProvider()
	if err != nil {
		log.With("err", err).Error("creating metrics provider")
		os.Exit(1)
	}
	receiver, _, _ := sharedDependencies(ctx, log, meter)

	err = naisd.Upgrade(ctx, receiver, log.With("subsystem", "self-upgrade"))
	if err != nil {
		log.With("err", err).Error("upgrading naisd")
		os.Exit(1)
	}

	// We sleep a few seconds to let possible requests finish (e.g. status report pubsub)
	time.Sleep(3 * time.Second)

	log.Info("Done")
}

func sharedDependencies(ctx context.Context, log *slog.Logger, meter metric.Meter) (*naisd.DeployManager, naisd.HelmClient, *message.Publisher[message.Status]) {
	deployClient, err := pubsub.NewClient(ctx, cfg.EnvProjectID)
	if err != nil {
		log.With("err", err).Error("setting up new pub/sub client")
		os.Exit(1)
	}

	deploySubscriber := message.NewSubscriber[message.DeployInstruction](deployClient, cfg.EnvProjectID, cfg.DeploySubscription, log.With("subsystem", "instruction-subscriber"))
	statusPublisher := message.NewPublisher[message.Status](
		deployClient,
		cfg.NaisProjectID,
		naisStatusTopic,
		log.With("subsystem", "status-pubsub"),
		message.WithWaithForPublish(),
		message.WithAttributes(map[string]string{
			"tenant":      cfg.TenantName,
			"environment": cfg.Env,
		}),
	)
	statusPublisher.SetMeter(meter)

	kubeConfig := local.RESTConfig()

	localHelm := local.NewHelmClient(log.With("subsystem", "executor"), cfg.MockFailing)
	var executor naisd.Exec = localHelm
	var helmClient naisd.HelmClient = localHelm
	k8sClient := local.NewKubernetesClient()
	if cfg.Production {
		executor = &naisd.Executor{}

		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			log.With("err", err).Error("failed to get kubeconfig")
			os.Exit(1)
		}
		helmClient = helm.New(kubeConfig, "nais-system", log.With("subsystem", "helm"))
		k8sClient, err = kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			log.With("err", err).Error("setting up k8s client")
			os.Exit(1)
		}
		err := ensureAnnotation(ctx, k8sClient, cfg.EnvProjectID)
		if err != nil {
			log.With("err", err).Error("annotating namespace")
		}
	}
	receiver, err := naisd.NewDeployManager(
		deploySubscriber,
		statusPublisher,
		cfg.TenantName,
		cfg.Env,
		executor,
		k8sClient,
		kubeConfig,
		os.Getenv("NAIS_SA_NAME"),
		cfg.NaisProjectID,
		log.With("subsystem", "deploy"),
	)
	if err != nil {
		log.With("err", err).Error("setting up worker")
		os.Exit(1)
	}
	receiver.SetMeter(meter)

	return receiver, helmClient, statusPublisher
}

func newNaisdMetricsProvider() (metric.Meter, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	prefixView := func(i metricsdk.Instrument) (metricsdk.Stream, bool) {
		return metricsdk.Stream{Name: "naisd_" + i.Name}, true
	}

	return metricsdk.
		NewMeterProvider(
			metricsdk.WithReader(exporter),
			metricsdk.WithView(prefixView),
		).
		Meter("github.com/nais/fasit/naisd"), nil
}
