package main

import (
	"cloud.google.com/go/pubsub"
	"context"
	"flag"
	"fmt"
	"github.com/nais/fasit/pkg/status"
	"github.com/nais/fasit/pkg/workers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
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
	flag.StringVar(&cfg.StatusTopicID, "topic-id", "fasit-subscription", "Pub/sub topic for status")
	flag.StringVar(&cfg.PartnerName, "partner-name", "", "partner name")
	flag.StringVar(&cfg.Env, "env", "", "cluster environment")
}

func main() {
	flag.Parse()
	log := newLogger()
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up new pub/sub client")
	}

	statusPublisher := status.NewPublisher[status.Update](client, cfg.StatusTopicID)
	deploySubscriber := status.NewSubscriber[status.DeployInstruction](client, fmt.Sprintf("naisd-%v-%v", cfg.PartnerName, cfg.Env))

	receiver := workers.NewReceiver(statusPublisher, repo, log.WithField("subsystem", "status"))
	go receiver.Run(ctx)

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
