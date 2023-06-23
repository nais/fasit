package naisd

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/naisd/selfupgrade"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var getEnvironment = os.Environ

type DeploymentReceiver interface {
	Name() string
	Synchronous()
	Receive(ctx context.Context, f func(ctx context.Context, msg message.DeployInstruction) error) error
}

type StatusPublisher interface {
	Publish(ctx context.Context, msg message.Status) error
}

type file interface {
	io.WriteCloser
	Name() string
}

type DeployManager struct {
	deployments           DeploymentReceiver
	statuses              StatusPublisher
	kubeConfig            *rest.Config
	k8sClient             kubernetes.Interface
	k8sServiceAccountName string
	naisProjectID         string
	log                   *logrus.Entry
	helmCache             string
	env                   string
	tenantName            string
	executor              Exec
	createTempFile        func(string, string) (file, error)

	performNaisdUpgrades bool
	stop                 context.CancelFunc
}

func NewDeployManager(
	deploySubscriber DeploymentReceiver,
	statusPublisher StatusPublisher,
	tenantName, env string,
	executor Exec,
	k8sClient kubernetes.Interface,
	kubeConfig *rest.Config,
	k8sServiceAccountName,
	naisProjectID string,
	log *logrus.Entry,
) (*DeployManager, error) {
	helmCache, err := os.MkdirTemp(os.TempDir(), "naisd-helm-*")
	if err != nil {
		return nil, err
	}

	receiver := &DeployManager{
		deployments:           deploySubscriber,
		statuses:              statusPublisher,
		log:                   log,
		helmCache:             helmCache,
		env:                   env,
		tenantName:            tenantName,
		executor:              executor,
		kubeConfig:            kubeConfig,
		k8sClient:             k8sClient,
		naisProjectID:         naisProjectID,
		k8sServiceAccountName: k8sServiceAccountName,
		createTempFile:        func(prefix, suffix string) (file, error) { return os.CreateTemp(prefix, suffix) },
	}

	return receiver, nil
}

func (d *DeployManager) Run(ctx context.Context) {
	d.log.WithField("subscription", d.deployments.Name()).Info("Starting deploy receiver")
	d.deployments.Synchronous()
	ctx, d.stop = context.WithCancel(ctx)
	err := d.deployments.Receive(ctx, d.handler)
	if err != nil {
		d.log.WithError(err).Error("receive status messages")
		// retry logic? This should only trigger when an upgrade is triggered.
	}
}

func (d *DeployManager) handler(ctx context.Context, msg message.DeployInstruction) error {
	d.log.WithFields(logrus.Fields{
		"name":    msg.Name,
		"chart":   msg.Chart,
		"version": msg.Version,
	}).Debug("Received instruction")

	if msg.Name == "naisd" && !d.performNaisdUpgrades {
		d.log.Debug("Offloading naisd upgrade")
		err := selfupgrade.StartJob(ctx, d.k8sClient, msg, d.naisProjectID, d.env, d.tenantName)
		if err != nil {
			return err
		}

		// Some hacks to try to reduce number of upgrades.
		message.ForceAck(ctx)
		time.Sleep(1 * time.Second)
		d.stop()
		return nil
	}

	valuesFile, err := d.makeHelmValues(msg)
	if err != nil {
		return err
	}
	defer os.Remove(valuesFile)

	args, err := helmArgs(msg, valuesFile)
	if err != nil {
		return err
	}

	helmStatus := message.Helm{
		Name:          msg.Name,
		Version:       msg.Version,
		ConfigHash:    msg.ConfigHash,
		RolloutStatus: model.RolloutStatusPending,
	}

	_ = d.publishStatus(ctx, helmStatus)

	helmStatus.Log, err = d.runHelm(ctx, args)
	if err != nil {
		log.Printf("failed to run helm %s: %s", msg.Name, err)
		helmStatus.RolloutStatus = model.RolloutStatusFailed
	} else {
		helmStatus.RolloutStatus = model.RolloutStatusDeployed
	}

	return d.publishStatus(ctx, helmStatus)
}

func (d *DeployManager) publishStatus(ctx context.Context, msg message.Helm) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	statusUpdate := message.Status{
		Tenant:      d.tenantName,
		Environment: d.env,
		Type:        message.StatusTypeHelm,
		Data:        data,
	}

	return d.statuses.Publish(ctx, statusUpdate)
}

func (d *DeployManager) runHelm(ctx context.Context, args []string) (string, error) {
	connectionFlags := []string{
		"--kube-apiserver",
		d.kubeConfig.Host,
		"--kube-ca-file",
		d.kubeConfig.CAFile,
		"--kube-token",
		d.kubeConfig.BearerToken,
	}

	helmFlags := []string{
		"--atomic",
		"--cleanup-on-fail",
		"--debug",
	}

	helmArgs := append(args, append(connectionFlags, helmFlags...)...)

	environment := append(getEnvironment(),
		"HELM_CACHE_HOME="+d.helmCache,
	)

	buf := &timeLogger{}

	cmd := exec.CommandContext(ctx, "helm", helmArgs...)
	cmd.Env = append(cmd.Env, environment...)
	cmd.Stdout = io.MultiWriter(buf, os.Stdout)
	cmd.Stderr = io.MultiWriter(buf, os.Stderr)

	err := d.executor.Execute(cmd)
	return buf.String(), err
}

func (d *DeployManager) makeHelmValues(m message.DeployInstruction) (string, error) {
	file, err := d.createTempFile("", "values-*.yaml")
	if err != nil {
		return "", err
	}
	defer file.Close()

	enc := yaml.NewEncoder(file)
	if err := enc.Encode(m.Values); err != nil {
		return "", err
	}

	return file.Name(), nil
}

func helmArgs(m message.DeployInstruction, valuesFile string) ([]string, error) {
	timeout := 5 * time.Minute
	if m.Timeout.Seconds() > 10 {
		timeout = m.Timeout
	}

	namespace := "nais-system"
	if strings.HasPrefix(m.Name, "kyverno") {
		namespace = "kyverno"
	}

	args := []string{
		"upgrade",
		"--atomic",
		"--install",
		m.Name,
		m.Chart,
		"--namespace",
		namespace,
		"--create-namespace",
		"--version",
		m.Version,
		"-f",
		valuesFile,
		"--timeout",
		timeout.String(),
	}

	return args, nil
}
