package local

import (
	"math/rand/v2"

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

type helmClient struct{}

func NewHelmClient() naisd.HelmClient {
	return &helmClient{}
}

func (h *helmClient) List() ([]*release.Release, error) {
	res := []*release.Release{}
	features := []string{"up", "naisd"}
	for i, feature := range features {
		r := &release.Release{
			Name:    feature,
			Version: i,
			Info: &release.Info{
				LastDeployed: htime.Now(),
				Status:       statuses[rand.IntN(len(statuses))], // #nosec G404
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
