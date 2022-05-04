package local

import (
	"math/rand"
	"time"

	"github.com/nais/fasit/pkg/naisd"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	htime "helm.sh/helm/v3/pkg/time"
)

var statuses = []release.Status{
	release.StatusUnknown,
	release.StatusDeployed,
	release.StatusUninstalled,
	release.StatusSuperseded,
	release.StatusFailed,
	release.StatusUninstalling,
	release.StatusPendingInstall,
	release.StatusPendingUpgrade,
	release.StatusPendingRollback,
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

type helmClient struct{}

func NewHelmClient() naisd.HelmClient {
	return &helmClient{}
}

func (h *helmClient) List() ([]*release.Release, error) {
	res := []*release.Release{}
	features := []string{"up", "naisd"}
	for _, feature := range features {
		r := &release.Release{
			Name:    feature,
			Version: rand.Intn(100),
			Info: &release.Info{
				LastDeployed: htime.Now(),
				Status:       statuses[rand.Intn(len(statuses))],
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "0.0.1",
				},
			},
		}

		res = append(res, r)
	}
	return res, nil
}
