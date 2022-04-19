package workers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"
)

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
	deployments    DeploymentReceiver
	statuses       StatusPublisher
	kubeConfig     *rest.Config
	log            *logrus.Entry
	helmCache      string
	env            string
	partnerName    string
	executor       Exec
	createTempFile func(string, string) (file, error)
}

func NewDeployManager(
	deploySubscriber DeploymentReceiver,
	statusPublisher StatusPublisher,
	partnerName, env string,
	executor Exec,
	kubeConfig *rest.Config,
	log *logrus.Entry,
) (*DeployManager, error) {
	helmCache, err := os.MkdirTemp(os.TempDir(), "naisd-helm-*")
	if err != nil {
		return nil, err
	}

	receiver := &DeployManager{
		deployments:    deploySubscriber,
		statuses:       statusPublisher,
		log:            log,
		helmCache:      helmCache,
		env:            env,
		partnerName:    partnerName,
		executor:       executor,
		kubeConfig:     kubeConfig,
		createTempFile: func(prefix, suffix string) (file, error) { return os.CreateTemp(prefix, suffix) },
	}

	return receiver, nil
}

func (d *DeployManager) Run(ctx context.Context) {
	d.log.WithField("subscription", d.deployments.Name()).Info("Starting deploy receiver")
	d.deployments.Synchronous()
	err := d.deployments.Receive(ctx, d.handler)
	if err != nil {
		d.log.WithError(err).Error("receive status messages")
		// retry logic, kanskje. Denne skal aldri trigge
	}
}

func (d *DeployManager) handler(ctx context.Context, msg message.DeployInstruction) error {
	d.log.WithFields(logrus.Fields{
		"name":    msg.Name,
		"chart":   msg.Chart,
		"version": msg.Version,
	}).Debug("Received instruction")

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
		RolloutStatus: "ok",
	}

	if err := d.runHelm(ctx, args); err != nil {
		log.Printf("failed to run helm %s: %s", msg.Name, err)
		helmStatus.RolloutStatus = "failed"
	}

	data, err := json.Marshal(helmStatus)
	if err != nil {
		return err
	}

	statusUpdate := message.Status{
		Partner:     d.partnerName,
		Environment: d.env,
		Type:        message.StatusTypeHelm,
		Data:        data,
	}

	return d.statuses.Publish(ctx, statusUpdate)
}

func (d *DeployManager) runHelm(ctx context.Context, args []string) error {
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
	}

	helmArgs := append(args, append(connectionFlags, helmFlags...)...)

	environment := []string{
		"HELM_EXPERIMENTAL_OCI=1",
		"HELM_CACHE_HOME=" + d.helmCache,
	}

	cmd := exec.CommandContext(ctx, "helm", helmArgs...)
	cmd.Env = append(cmd.Env, environment...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return d.executor.Execute(cmd)
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
	args := []string{
		"upgrade",
		"--atomic",
		"--install",
		m.Name,
		m.Chart,
		"--namespace",
		"nais-system",
		"--create-namespace",
		"--version",
		m.Version,
		"-f",
		valuesFile,
	}

	if m.Repo != "" {
		args = append(args, "--repo", m.Repo)
	}

	return args, nil
}
