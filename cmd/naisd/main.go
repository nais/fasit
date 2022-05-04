package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/helm"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/naisd"
	"github.com/nais/fasit/pkg/workers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

const (
	deploySubscriptionID = "naisd-subscription"
	naisStatusTopic      = "status"
)

var (
	cfg     = DefaultConfig()
	envKind string

	promErrs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "naisd",
		Name:      "errors",
	}, []string{"location"})
)

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.EnvProjectID, "env-project-id", "local-test-dev", "Google project ID")
	flag.StringVar(&cfg.NaisProjectID, "nais-project-id", "nais-local-dev", "Nais project ID")
	flag.StringVar(&cfg.TenantName, "tenant-name", "test", "tenant name")
	flag.StringVar(&envKind, "cluster-kind", "", "management or tenant")
	flag.StringVar(&cfg.Env, "env", "dev", "cluster environment")
	flag.BoolVar(&cfg.Production, "production", false, "When in production, actually run helm install")
}

func main() {
	flag.Parse()
	log := newLogger()
	cfg.Kind = model.EnvironmentKind(envKind)
	if !cfg.Kind.IsValid() {
		log.Fatal("cluster kind not set")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	deployClient, err := pubsub.NewClient(ctx, cfg.EnvProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up new pub/sub client")
	}

	deploySubscriber := message.NewSubscriber[message.DeployInstruction](deployClient, cfg.EnvProjectID, deploySubscriptionID)
	statusPublisher := message.NewPublisher[message.Status](deployClient, cfg.NaisProjectID, naisStatusTopic, log.WithField("subsystem", "status-pubsub"))

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.WithError(err).Fatal("failed to get kubeconfig")
	}

	var executor naisd.Exec = &naisd.MockExecutor{Logger: log.WithField("subsystem", "executor")}
	if cfg.Production {
		executor = &naisd.Executor{}
	}
	receiver, err := naisd.NewDeployManager(deploySubscriber, statusPublisher, cfg.TenantName, cfg.Env, executor, kubeConfig, log.WithField("subsystem", "deploy"))
	if err != nil {
		log.WithError(err).Fatal("setting up worker")
	}

	helmClient := helm.New(kubeConfig, "nais-system", log.WithField("subsystem", "helm"))

	s := workers.NewScheduler(log.WithField("subsystem", "scheduler"))
	helmListReporter := naisd.NewStatusReporter(cfg.TenantName, cfg.Env, helmClient, statusPublisher)
	healthReporter := naisd.NewHealthReporter(cfg.TenantName, cfg.Env, cfg.Kind, statusPublisher)
	s.Register("helm-list", helmListReporter, 15*time.Minute)
	s.Register("health", healthReporter, 1*time.Minute)
	s.Start(ctx)

	log.Info("Receiver started")
	receiver.Run(ctx)
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
