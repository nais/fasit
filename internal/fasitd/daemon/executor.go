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
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
)

const defaultHelmTimeout = 5 * time.Minute

// maxHistory matches the Helm CLI's default --history-max so release secrets
// do not accumulate without bound.
const maxHistory = 10

type Executor interface {
	InstallOrUpgrade(ctx context.Context, req DeployRequest, logFn func(string)) error
	Uninstall(ctx context.Context, name string, timeout time.Duration, logFn func(string)) error
}

type DeployRequest struct {
	Name    string
	Chart   string
	Version string
	Values  map[string]any
	Timeout time.Duration
	// Atomic rolls the release back if the upgrade does not become ready.
	Atomic bool
}

type HelmExecutor struct {
	restConfig *rest.Config
	namespace  string
	log        *slog.Logger
}

var _ Executor = (*HelmExecutor)(nil)

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

	timeout := helmTimeout(req.Timeout)

	installed, err := e.prepareRelease(cfg, req.Name, timeout, logFn)
	if err != nil {
		return err
	}

	if !installed {
		install := action.NewInstall(cfg)
		install.ReleaseName = req.Name
		install.Namespace = e.namespace
		install.Wait = true
		install.Atomic = req.Atomic
		install.Timeout = timeout
		_, err = install.RunWithContext(ctx, ch, req.Values)
		return err
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = e.namespace
	upgrade.Wait = true
	upgrade.Atomic = req.Atomic
	upgrade.Timeout = timeout
	upgrade.MaxHistory = maxHistory
	_, err = upgrade.RunWithContext(ctx, req.Name, ch, req.Values)
	return err
}

// prepareRelease reports whether name is currently installed. A release left in
// a pending state by an operation that was killed midway (for example a fasitd
// pod replaced during its own upgrade) blocks every later operation, so once it
// is older than timeout it is recovered: rolled back to the previous revision,
// or removed if it never finished installing. A younger pending release is
// treated as an operation still in progress.
func (e *HelmExecutor) prepareRelease(cfg *action.Configuration, name string, timeout time.Duration, logFn func(string)) (bool, error) {
	hist := action.NewHistory(cfg)
	hist.Max = 1
	releases, err := hist.Run(name)
	if errors.Is(err, driver.ErrReleaseNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check release history: %w", err)
	}
	if len(releases) == 0 {
		return false, nil
	}

	last := releases[len(releases)-1]
	if !last.Info.Status.IsPending() {
		return true, nil
	}
	if age := time.Since(last.Info.LastDeployed.Time); age < timeout {
		return false, fmt.Errorf("release %s has an operation in progress (%s for %s)", name, last.Info.Status, age.Round(time.Second))
	}

	logFn(fmt.Sprintf("recovering release %s stuck in %s since %s", name, last.Info.Status, last.Info.LastDeployed.Format(time.RFC3339)))
	if last.Info.Status == release.StatusPendingInstall || last.Version <= 1 {
		if _, err := newUninstall(cfg, timeout).Run(name); err != nil {
			return false, fmt.Errorf("remove stuck release %s: %w", name, err)
		}
		return false, nil
	}

	rollback := action.NewRollback(cfg)
	rollback.Wait = true
	rollback.Timeout = timeout
	rollback.MaxHistory = maxHistory
	if err := rollback.Run(name); err != nil {
		return false, fmt.Errorf("roll back stuck release %s: %w", name, err)
	}
	return true, nil
}

func (e *HelmExecutor) Uninstall(_ context.Context, name string, timeout time.Duration, logFn func(string)) error {
	cfg, err := e.newActionConfig(logFn)
	if err != nil {
		return err
	}

	uninstall := newUninstall(cfg, helmTimeout(timeout))
	uninstall.IgnoreNotFound = true
	_, err = uninstall.Run(name)
	return err
}

func newUninstall(cfg *action.Configuration, timeout time.Duration) *action.Uninstall {
	uninstall := action.NewUninstall(cfg)
	uninstall.Wait = true
	uninstall.Timeout = timeout
	return uninstall
}

func helmTimeout(requested time.Duration) time.Duration {
	if requested <= 0 {
		return defaultHelmTimeout
	}
	return requested
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
