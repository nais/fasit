package workers

import (
	"context"
	"github.com/nais/fasit/pkg/status"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type DeployReceiver struct {
	manager    *status.Subscriber[status.DeployInstruction]
	kubeConfig *rest.Config
	log        *logrus.Entry
	helmCache  string
}

func NewDeployReceiver(mgr *status.Subscriber[status.DeployInstruction], log *logrus.Entry) (*DeployReceiver, error) {
	helmCache, err := os.MkdirTemp(os.TempDir(), "naisd-helm-*")
	if err != nil {
		return nil, err
	}

	receiver := &DeployReceiver{manager: mgr, log: log, helmCache: helmCache}
	return receiver, nil
}

func (r *DeployReceiver) Run(ctx context.Context) {
	err := r.manager.Receive(ctx, r.handler)
	if err != nil {
		r.log.WithError(err).Error("receive status messages")
		// retry logic, kanskje. Denne skal aldri trigge
	}
}

func (r *DeployReceiver) handler(ctx context.Context, message status.DeployInstruction) error {
	args, err := helmArgs(message)
	if err != nil {
		return err
	}
	if err := r.runHelm(ctx, args); err != nil {
		log.Printf("failed to run helm %s: %s", message.Name, err)
		return nil
	}
	return nil
}

func (d *DeployReceiver) runHelm(ctx context.Context, args []string) error {
	baseFlags := []string{
		"--kube-apiserver",
		d.kubeConfig.Host,
		"--kube-ca-file",
		d.kubeConfig.CAFile,
		"--kube-token",
		d.kubeConfig.BearerToken,
	}

	environment := []string{
		"HELM_EXPERIMENTAL_OCI=1",
		"HELM_CACHE_HOME=" + d.helmCache,
	}

	cmd := exec.CommandContext(ctx, "helm", append(baseFlags, args...)...)
	cmd.Env = append(cmd.Env, environment...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func helmArgs(m status.DeployInstruction) ([]string, error) {
	file, err := os.CreateTemp("", "values-*.yaml")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	enc := yaml.NewEncoder(file)
	if err := enc.Encode(m.Values); err != nil {
		return nil, err
	}

	args := []string{
		"upgrade",
		"--install",
		m.Name,
		m.Chart,
		"--namespace",
		"nais-system",
		"--create-namespace",
		"--version",
		m.Version,
		"-f",
		filepath.Join(os.TempDir(), file.Name()),
	}

	if m.Repo != "" {
		args = append(args, "--repo", m.Repo)
	}

	return args, nil
}
