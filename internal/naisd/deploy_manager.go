package naisd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisd/selfupgrade"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	errRestartRequired = errors.New("restart required")
	errPostpone        = fmt.Errorf("postpone: %w", message.ErrNack)
	getEnvironment     = os.Environ
)

type lockedInt struct {
	sync.Mutex
	value int
}

func (i *lockedInt) Inc() {
	i.Lock()
	defer i.Unlock()
	i.value++
}

func (i *lockedInt) Dec() {
	i.Lock()
	defer i.Unlock()
	i.value--
	if i.value < 0 {
		slog.Info("negative progress count, resetting to 0")
		i.value = 0
	}
}

func (i *lockedInt) Value() int {
	i.Lock()
	defer i.Unlock()
	return i.value
}

type lockedTime struct {
	sync.Mutex
	value time.Time
}

func (t *lockedTime) Set() {
	t.Lock()
	defer t.Unlock()
	t.value = time.Now()
}

func (t *lockedTime) Get() time.Time {
	t.Lock()
	defer t.Unlock()
	return t.value
}

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
	log                   *slog.Logger
	helmCache             string
	env                   string
	tenantName            string
	executor              Exec
	createTempFile        func(string, string) (file, error)
	runOnlyOnce           bool
	RepublishHelmList     func()

	// We need to keep track of how many deployments are in progress, so that we can
	// postpone the upgrade of naisd until all other deployments are done.
	inProgress  lockedInt
	lastChecked lockedTime

	performNaisdUpgrades bool
	stop                 context.CancelFunc

	helmDuration    metric.Float64Histogram
	handlerDuration metric.Float64Histogram
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
	log *slog.Logger,
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
		RepublishHelmList:     func() {},
	}

	return receiver, nil
}

func (d *DeployManager) SetMeter(meter metric.Meter) {
	var err error
	d.helmDuration, err = meter.Float64Histogram("helm_execution_duration_seconds",
		metric.WithDescription("Helm command execution duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		d.log.With("err", err).Warn("failed to create helm duration histogram")
	}

	d.handlerDuration, err = meter.Float64Histogram("deploy_handler_duration_seconds",
		metric.WithDescription("Deploy handler total duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		d.log.With("err", err).Warn("failed to create handler duration histogram")
	}
}

func (d *DeployManager) Run(ctx context.Context) {
	d.log.With("subscription", d.deployments.Name()).Info("Starting deploy receiver")
	d.deployments.Synchronous()

	ctx, d.stop = context.WithCancel(ctx)
	for {
		err := d.deployments.Receive(ctx, d.handler)
		if err != nil {
			if errors.Is(err, errRestartRequired) {
				d.log.Info("Restarting deploy receiver")
				return
			}
			d.log.With("err", err).Error("receive status messages")
			// retry logic? This should only trigger when an upgrade is triggered.
		}

		if d.runOnlyOnce {
			break
		}

		select {
		case <-ctx.Done():
			d.log.Info("Stopping deploy receiver")
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (d *DeployManager) handler(ctx context.Context, msg message.DeployInstruction) error {
	// Force ack message to prevent retries for long running tasks
	message.ForceAck(ctx)

	// Detach from the pubsub context so in-flight work is not cancelled
	// when the subscription stops accepting new messages on shutdown.
	ctx = context.WithoutCancel(ctx)

	handlerStart := time.Now()
	defer func() {
		if d.handlerDuration != nil {
			d.handlerDuration.Record(ctx, time.Since(handlerStart).Seconds(), metric.WithAttributes(
				attribute.String("feature", msg.Name),
			))
		}
	}()

	if msg.Uninstall {
		return d.uninstallHandler(ctx, msg)
	}

	d.log.With("name", msg.Name,
		"chart", msg.Chart,
		"version", msg.Version).Debug("Received instruction")

	pubsubLog := newPubsubLogger(msg.ID, d.statuses, d.log)

	go pubsubLog.Run(ctx)
	defer ioconvenience.CloseWithLog(pubsubLog, d.log)

	if msg.Name == "naisd" && !d.performNaisdUpgrades {
		if d.inProgress.Value() > 0 {
			if d.lastChecked.Get().Add(15 * time.Second).Before(time.Now()) {
				d.log.With("inProgress", d.inProgress.Value()).Debug("Postponing naisd upgrade")
				_, _ = fmt.Fprintf(pubsubLog, "Postponing naisd upgrade, %d deployments in progress\n", d.inProgress.Value())
				d.lastChecked.Set()
			}
			return errPostpone
		}

		d.log.Debug("Offloading naisd upgrade")
		if _, ok := d.executor.(*MockExecutor); ok {
			_, _ = fmt.Fprintln(pubsubLog, "MockExecutor, not starting regular naisd upgrade")
		} else {
			err := selfupgrade.StartJob(ctx, d.k8sClient, msg, d.naisProjectID, d.env, d.tenantName)
			if err != nil {
				return err
			}

			time.Sleep(1 * time.Second)
			d.stop()
			return errRestartRequired
		}
	}

	d.inProgress.Inc()
	defer d.inProgress.Dec()

	valuesFile, err := d.makeHelmValues(msg)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(valuesFile); err != nil {
			d.log.With("err", err).Error("error removing values file")
		}
	}()

	namespace := namespaceFor(msg.Name)

	args, err := helmUpgradeArgs(msg, namespace, valuesFile)
	if err != nil {
		return err
	}

	helmStatus := message.Helm{
		DIID:          msg.ID,
		ConfigHash:    msg.ConfigHash,
		RolloutStatus: feature.DeployStatusInstalling,
	}

	_ = d.publishStatus(ctx, helmStatus)

	eventsCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go d.listenForEvents(eventsCtx, pubsubLog, msg, namespace)

	helmStart := time.Now()
	helmStatus.Log, err = d.runHelm(ctx, pubsubLog, args)
	if err != nil {
		d.log.With("err", err, "feature", msg.Name).Warn("failed to run helm")
		helmStatus.RolloutStatus = feature.DeployStatusFailed
		helmStatus.Error = err.Error()
	} else {
		helmStatus.RolloutStatus = feature.DeployStatusDeployed
	}
	if d.helmDuration != nil {
		d.helmDuration.Record(ctx, time.Since(helmStart).Seconds(), metric.WithAttributes(
			attribute.String("feature", msg.Name),
			attribute.String("status", helmStatus.RolloutStatus.String()),
		))
	}

	d.RepublishHelmList()

	return d.publishStatus(ctx, helmStatus)
}

func (d *DeployManager) listenForEvents(ctx context.Context, pubsubLog *pubsubLogger, msg message.DeployInstruction, namespace string) {
	d.log.Debug("Start listen for events")
	if d.k8sClient == nil {
		d.log.Warn("k8sClient is nil")
		// This should only happen in tests, but is not a critical state either way.
		return
	}

	opts := metav1.ListOptions{}
	watcher, err := d.k8sClient.CoreV1().Events(namespace).Watch(ctx, opts)
	if err != nil {
		_, _ = fmt.Fprintf(pubsubLog, "failed to watch events: %s\n", err)
		d.log.With("err", err).Warn("failed to watch events")
		return
	}

	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Debug("Stop listen for events, context done")
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// channel closed
				d.log.Debug("Stop listen for events, channel closed")
				return
			}
			if event.Type != watch.Added {
				// we only care about new events
				continue
			}

			e, ok := event.Object.(*corev1.Event)
			if !ok {
				d.log.Warn("Ignore event, not an event")
				// not an event
				continue
			}

			if e.Type != "Error" && e.Type != "Warning" {
				// we only care about errors and warnings
				continue
			}

			if strings.Contains(e.InvolvedObject.Name, msg.Name) {
				d.log.With("event", e.Message).Info("Add event")
				pubsubLog.AddEvent(e)
			}
		}
	}
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

func (d *DeployManager) runHelm(ctx context.Context, pubsubLog *pubsubLogger, args []string) (string, error) {
	helmArgs := append(args, d.connectionFlags()...)
	if _, ok := os.LookupEnv("DEBUG"); ok {
		helmArgs = append(helmArgs, "--debug")
	}

	environment := append(getEnvironment(),
		"HELM_CACHE_HOME="+d.helmCache,
	)

	cmd := exec.CommandContext(ctx, "helm", helmArgs...) // #nosec G204
	cmd.Env = append(cmd.Env, environment...)
	if pubsubLog != nil {
		cmd.Stdout = io.MultiWriter(pubsubLog, os.Stdout)
		cmd.Stderr = io.MultiWriter(pubsubLog, os.Stderr)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := d.executor.Execute(cmd)
	return "", err
}

func (d *DeployManager) makeHelmValues(m message.DeployInstruction) (string, error) {
	file, err := d.createTempFile("", "values-*.yaml")
	if err != nil {
		return "", err
	}
	defer ioconvenience.CloseWithLog(file, d.log)

	enc := yaml.NewEncoder(file)
	if err := enc.Encode(m.Values); err != nil {
		return "", err
	}

	return file.Name(), nil
}

func (d *DeployManager) runOnce() {
	d.runOnlyOnce = true
}

func (d *DeployManager) uninstallHandler(ctx context.Context, msg message.DeployInstruction) error {
	log := d.log.With("feature", msg.Name, "method", "uninstall")
	if msg.Name == "naisd" || msg.Name == "fasitd" || msg.Name == "" {
		log.Warn("will not uninstall")
		return nil
	}

	pubsubLog := newPubsubLogger(msg.ID, d.statuses, d.log)
	go pubsubLog.Run(ctx)
	defer ioconvenience.CloseWithLog(pubsubLog, d.log)

	timeout := 5 * time.Minute
	if msg.Timeout.Seconds() > 10 {
		timeout = msg.Timeout
	}

	args := []string{
		"uninstall",
		"--namespace",
		namespaceFor(msg.Name),
		"--timeout", timeout.String(),
		msg.Name,
	}

	_, err := d.runHelm(ctx, pubsubLog, args)

	d.RepublishHelmList()

	return err
}

func (d *DeployManager) connectionFlags() []string {
	return []string{
		"--kube-apiserver",
		d.kubeConfig.Host,
		"--kube-ca-file",
		d.kubeConfig.CAFile,
		"--kube-token",
		d.kubeConfig.BearerToken,
	}
}

func helmUpgradeArgs(m message.DeployInstruction, namespace, valuesFile string) ([]string, error) {
	timeout := 5 * time.Minute
	if m.Timeout.Seconds() > 10 {
		timeout = m.Timeout
	}
	args := []string{
		"upgrade",
		"--atomic",
		"--cleanup-on-fail",
		"--history-max", "10",
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

func namespaceFor(featureName string) string {
	if strings.HasPrefix(featureName, "kyverno") {
		return "kyverno"
	}
	return "nais-system"
}
