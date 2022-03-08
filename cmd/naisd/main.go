package main

import (
	"cloud.google.com/go/pubsub"
	"context"
	"flag"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/workers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
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
	flag.StringVar(&cfg.GCPProjectID, "project-id", "banankake", "Google project ID")
	flag.StringVar(&cfg.DeploySubscriptionID, "deploy-subscription-id", "naisd-subscription", "Pub/sub subscription for deploys")
	flag.StringVar(&cfg.StatusTopicRef, "status-topic-ref", "projects/banankake/topic/status)", "status topic ref (projects/<project_id>/topic/<topic_id>)")
	flag.StringVar(&cfg.PartnerName, "partner-name", "", "partner name")
	flag.StringVar(&cfg.Env, "env", "", "cluster environment")
}

func main() {
	flag.Parse()
	log := newLogger()
	ctx := context.Background()
	deployClient, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up new pub/sub client")
	}

	deploySubscriber := message.NewSubscriber[message.DeployInstruction](deployClient, cfg.DeploySubscriptionID)
	statusPublisher := message.NewPublisher[message.Status](deployClient, cfg.StatusTopicRef)

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		//log.WithError(err).Fatal("failed to get kubeconfig")
		kubeConfig = &rest.Config{}
	}

	receiver, err := workers.NewDeployManager(deploySubscriber, statusPublisher, cfg.PartnerName, cfg.Env, &workers.MockExecutor{Logger: log.WithField("subsystem", "executor")}, kubeConfig, log.WithField("subsystem", "deploy"))
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
