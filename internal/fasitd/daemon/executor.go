package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nais/fasit/internal/helm"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
)

const defaultHelmTimeout = 5 * time.Minute

// Executor runs Helm operations for a fasitd command.
type Executor interface {
	InstallOrUpgrade(ctx context.Context, req DeployRequest, logFn func(string)) error
	Uninstall(ctx context.Context, name string, timeout time.Duration, logFn func(string)) error
}

// DeployRequest holds the parameters for a Helm install or upgrade.
type DeployRequest struct {
	Name    string
	Chart   string
	Version string
	Values  map[string]any
	Timeout time.Duration
	Atomic  bool
}

// HelmExecutor executes real Helm operations against a Kubernetes cluster.
type HelmExecutor struct {
	restConfig *rest.Config
	namespace  string
	log        *slog.Logger
}

func NewHelmExecutor(restConfig *rest.Config, namespace string, log *slog.Logger) *HelmExecutor {
	return &HelmExecutor{
		restConfig: restConfig,
		namespace:  namespace,
		log:        log.With("subsystem", "fasitd-helm"),
	}
}

func (e *HelmExecutor) newActionConfig(logFn func(string)) (*action.Configuration, error) {
	cfg := &action.Configuration{}
	debugLog := func(format string, v ...any) {
		msg := fmt.Sprintf(format, v...)
		e.log.Debug(msg)
		if logFn != nil {
			logFn(msg)
		}
	}
	if err := cfg.Init(&helm.K8sClient{Cfg: e.restConfig}, e.namespace, "", debugLog); err != nil {
		return nil, fmt.Errorf("init helm action config: %w", err)
	}
	return cfg, nil
}

func (e *HelmExecutor) InstallOrUpgrade(ctx context.Context, req DeployRequest, logFn func(string)) error {
	chartBuf, err := helm.DownloadChart(req.Chart, req.Version, "")
	if err != nil {
		return fmt.Errorf("download chart %s:%s: %w", req.Chart, req.Version, err)
	}
	ch, err := loader.LoadArchive(chartBuf)
	if err != nil {
		return fmt.Errorf("load chart archive: %w", err)
	}

	cfg, err := e.newActionConfig(logFn)
	if err != nil {
		return err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultHelmTimeout
	}

	histClient := action.NewHistory(cfg)
	histClient.Max = 1
	_, histErr := histClient.Run(req.Name)
	if errors.Is(histErr, driver.ErrReleaseNotFound) {
		install := action.NewInstall(cfg)
		install.ReleaseName = req.Name
		install.Namespace = e.namespace
		install.CreateNamespace = true
		install.Wait = true
		install.Atomic = req.Atomic
		install.Timeout = timeout
		_, err = install.RunWithContext(ctx, ch, req.Values)
		return err
	}
	if histErr != nil {
		return fmt.Errorf("check release history: %w", histErr)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = e.namespace
	upgrade.Install = true
	upgrade.Wait = true
	upgrade.Atomic = req.Atomic
	upgrade.Timeout = timeout
	upgrade.MaxHistory = 10
	_, err = upgrade.RunWithContext(ctx, req.Name, ch, req.Values)
	return err
}

func (e *HelmExecutor) Uninstall(_ context.Context, name string, timeout time.Duration, logFn func(string)) error {
	cfg, err := e.newActionConfig(logFn)
	if err != nil {
		return err
	}

	if timeout <= 0 {
		timeout = defaultHelmTimeout
	}

	uninstall := action.NewUninstall(cfg)
	uninstall.Wait = true
	uninstall.Timeout = timeout
	uninstall.IgnoreNotFound = true
	_, err = uninstall.Run(name)
	return err
}

func parseValues(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var vals map[string]any
	if err := json.Unmarshal(raw, &vals); err != nil {
		return nil, fmt.Errorf("unmarshal values: %w", err)
	}
	if vals == nil {
		vals = map[string]any{}
	}
	return vals, nil
}

// Ensure HelmExecutor implements Executor.
var _ Executor = (*HelmExecutor)(nil)
