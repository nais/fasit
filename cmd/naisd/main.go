package main

import (
	"context"
	"flag"

	"cloud.google.com/go/pubsub"
	"github.com/nais/fasit/pkg/message"
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
	cfg = DefaultConfig()

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
	flag.StringVar(&cfg.PartnerName, "partner-name", "test", "partner name")
	flag.StringVar(&cfg.Env, "env", "dev", "cluster environment")
	flag.BoolVar(&cfg.Production, "production", false, "When in production, actually run helm install")
}

func main() {
	flag.Parse()
	log := newLogger()
	ctx := context.Background()
	deployClient, err := pubsub.NewClient(ctx, cfg.EnvProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up new pub/sub client")
	}

	deploySubscriber := message.NewSubscriber[message.DeployInstruction](deployClient, cfg.EnvProjectID, deploySubscriptionID)
	statusPublisher := message.NewPublisher[message.Status](deployClient, cfg.NaisProjectID, naisStatusTopic)

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		// log.WithError(err).Fatal("failed to get kubeconfig")
		kubeConfig = &rest.Config{}
	}

	var executor workers.Exec = &workers.MockExecutor{Logger: log.WithField("subsystem", "executor")}
	if cfg.Production {
		executor = &workers.Executor{}
	}
	receiver, err := workers.NewDeployManager(deploySubscriber, statusPublisher, cfg.PartnerName, cfg.Env, executor, kubeConfig, log.WithField("subsystem", "deploy"))
	if err != nil {
		log.WithError(err).Fatal("setting up worker")
	}

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
