package local

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/nais/fasit/internal/naisd"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	htime "helm.sh/helm/v3/pkg/time"
)

type helmRecord struct {
	name         string
	version      string
	revision     int
	status       release.Status
	lastDeployed htime.Time
}

type HelmClient struct {
	logger        *logrus.Entry
	mockFailing   bool
	numSuccessful int

	mu       sync.Mutex
	releases map[string]*helmRecord
}

func NewHelmClient(logger *logrus.Entry, mockFailing bool) *HelmClient {
	return &HelmClient{
		logger:      logger,
		mockFailing: mockFailing,
		releases:    map[string]*helmRecord{},
	}
}

func (h *HelmClient) Mock() bool { return true }

func (h *HelmClient) List() ([]*release.Release, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	res := make([]*release.Release, 0, len(h.releases))
	for _, r := range h.releases {
		res = append(res, &release.Release{
			Name:    r.name,
			Version: r.revision,
			Info: &release.Info{
				LastDeployed: r.lastDeployed,
				Status:       r.status,
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: r.version,
				},
			},
		})
	}
	return res, nil
}

func (h *HelmClient) Execute(cmd *exec.Cmd) error {
	h.logger.Println(cmd.String())

	if cmd.Stdout != nil {
		fmt.Fprintln(cmd.Stdout, "Start mock executor", time.Now())
		defer fmt.Fprintln(cmd.Stdout, "end of mock executor")
	}
	time.Sleep(3 * time.Second)

	op, name, version := parseHelmCommand(cmd.Args)

	switch op {
	case "upgrade":
		h.recordUpgrade(name, version)
	case "uninstall":
		h.recordUninstall(name)
	}

	if h.mockFailing {
		if h.numSuccessful <= 0 {
			h.markFailed(name)
			return fmt.Errorf("execution failed")
		}
		h.numSuccessful--
	}

	return nil
}

func (h *HelmClient) recordUpgrade(name, version string) {
	if name == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	rec, ok := h.releases[name]
	if !ok {
		rec = &helmRecord{name: name}
		h.releases[name] = rec
	}
	rec.version = version
	rec.revision++
	rec.status = release.StatusDeployed
	rec.lastDeployed = htime.Now()
}

func (h *HelmClient) recordUninstall(name string) {
	if name == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.releases, name)
}

func (h *HelmClient) markFailed(name string) {
	if name == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if rec, ok := h.releases[name]; ok {
		rec.status = release.StatusFailed
		rec.lastDeployed = htime.Now()
	}
}

// parseHelmCommand extracts the operation and (for upgrade) the release name
// and chart version from a `helm <op> ...` argv. It mirrors the argument shape
// produced by deploy_manager.helmUpgradeArgs / uninstallArgs:
//
//	helm upgrade --atomic ... --install <name> <chart> --namespace <ns> --create-namespace --version <ver> ...
//	helm uninstall <name> --namespace <ns> ...
func parseHelmCommand(args []string) (op, name, version string) {
	if len(args) < 2 {
		return "", "", ""
	}
	op = args[1]
	switch op {
	case "upgrade":
		for i := 2; i < len(args); i++ {
			if args[i] == "--install" && i+1 < len(args) {
				name = args[i+1]
			}
			if args[i] == "--version" && i+1 < len(args) {
				version = args[i+1]
			}
		}
	case "uninstall":
		for i := 2; i < len(args); i++ {
			if args[i] == "--namespace" {
				break
			}
			if args[i] != "" && args[i][0] != '-' {
				name = args[i]
				break
			}
		}
	}
	return op, name, version
}

var _ naisd.HelmClient = (*HelmClient)(nil)
var _ naisd.Exec = (*HelmClient)(nil)
