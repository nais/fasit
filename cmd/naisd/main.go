package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/nais/fasit/cmd/naisd/local"
	"github.com/nais/fasit/pkg/helm"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/naisd"
	"github.com/nais/fasit/pkg/workers"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	deploySubscriptionID  = "naisd-subscription"
	consoleSubscriptionID = "naisd-console-subscription"
	naisStatusTopic       = "status"
)

var cfg = DefaultConfig()

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.EnvProjectID, "env-project-id", "local-test-dev", "Google project ID")
	flag.StringVar(&cfg.NaisProjectID, "nais-project-id", "nais-local-dev", "Nais project ID")
	flag.StringVar(&cfg.TenantName, "tenant-name", "test", "tenant name")
	flag.StringVar(&cfg.Env, "env", "dev", "cluster environment")
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
		if lvl, err := logrus.ParseLevel(envLvl); err != nil {
			log.WithError(err).Warn("log level not parsable")
		} else {
			log.SetLevel(lvl)
		}
	}

	if err := run(ctx, log); err != nil {
		log.Fatal(err)
	}

	log.Info("Run cancelled, exiting. If we've started an upgrade, we'll keep running until it's done.")
	select {
	case <-ctx.Done():
		log.Info("Shutting down")
	case <-time.After(10 * time.Minute):
		log.Fatal("Shutdown timed out")
	}
}

func run(ctx context.Context, log *logrus.Logger) error {
	receiver, helmClient, k8sClient, restConfig, deployClient, statusPublisher := sharedDependencies(ctx, log)

	s := workers.NewScheduler(log.WithField("subsystem", "scheduler"))
	helmListReporter := naisd.NewStatusReporter(cfg.TenantName, cfg.Env, helmClient, statusPublisher)
	healthReporter := naisd.NewHealthReporter(cfg.TenantName, cfg.Env, statusPublisher)
	kubernetesReporter := naisd.NewKubernetesReporter(cfg.TenantName, cfg.Env, k8sClient, statusPublisher)
	s.Register("helm-list", helmListReporter, 15*time.Minute)
	s.Register("health", healthReporter, 1*time.Minute)
	s.Register("kubernetes", kubernetesReporter, 3*time.Minute)
	s.Start(ctx)

	if !cfg.Management {
		namespaceSubscriber := message.NewSubscriber[message.Console](deployClient, cfg.EnvProjectID, consoleSubscriptionID)
		if cfg.Production {
			consoleMgr, err := naisd.NewConsoleManager(ctx, namespaceSubscriber, restConfig, cfg.EnvProjectID, cfg.Env, log.WithField("subsystem", "console"))
			if err != nil {
				return err
			}
			go consoleMgr.Run(ctx)
		}

	}

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

func upgrade(ctx context.Context, log *logrus.Logger) {
	log.Info("Upgrading naisd")
	receiver, _, _, _, _, _ := sharedDependencies(ctx, log)

	err := naisd.Upgrade(ctx, receiver, log.WithField("subsystem", "self-upgrade"))
	if err != nil {
		log.WithError(err).Fatal("upgrading naisd")
	}

	// We sleep a few seconds to let possible requests finish (e.g. status report pubsub)
	time.Sleep(3 * time.Second)

	log.Info("Done")
}

func sharedDependencies(ctx context.Context, log *logrus.Logger) (*naisd.DeployManager, naisd.HelmClient, kubernetes.Interface, *rest.Config, *pubsub.Client, *message.Publisher[message.Status]) {
	deployClient, err := pubsub.NewClient(ctx, cfg.EnvProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up new pub/sub client")
	}

	deploySubscriber := message.NewSubscriber[message.DeployInstruction](deployClient, cfg.EnvProjectID, deploySubscriptionID)
	statusPublisher := message.NewPublisher[message.Status](
		deployClient,
		cfg.NaisProjectID,
		naisStatusTopic,
		log.WithField("subsystem", "status-pubsub"),
		message.WithWaithForPublish(),
		message.WithAttributes(map[string]string{
			"tenant":      cfg.TenantName,
			"environment": cfg.Env,
		}),
	)

	kubeConfig := local.RESTConfig()

	var numSuccessful *int
	if cfg.MockFailing {
		numSuccessful = new(int)
	}

	var executor naisd.Exec = &naisd.MockExecutor{Logger: log.WithField("subsystem", "executor"), NumSuccessful: numSuccessful}
	helmClient := local.NewHelmClient()
	k8sClient := local.NewKubernetesClient()
	if cfg.Production {
		executor = &naisd.Executor{}

		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			log.WithError(err).Fatal("failed to get kubeconfig")
		}
		helmClient = helm.New(kubeConfig, "nais-system", log.WithField("subsystem", "helm"))
		k8sClient, err = kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			log.WithError(err).Fatal("setting up k8s client")
		}
		err := ensureAnnotation(ctx, k8sClient, cfg.EnvProjectID)
		if err != nil {
			log.WithError(err).Error("annotating namespace")
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
		log.WithField("subsystem", "deploy"),
	)
	if err != nil {
		log.WithError(err).Fatal("setting up worker")
	}

	return receiver, helmClient, k8sClient, kubeConfig, deployClient, statusPublisher
}
