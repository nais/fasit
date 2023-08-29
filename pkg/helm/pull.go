package helm

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

func DownloadChart(chart, version, repo string) (*bytes.Buffer, error) {
	client, err := registry.NewClient(registry.ClientOptHTTPClient(http.DefaultClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create registry client: %w", err)
	}

	sb := &strings.Builder{}
	downloader := downloader.ChartDownloader{
		Out:            sb,
		Verify:         downloader.VerifyNever,
		RegistryClient: client,
	}

	settings := &cli.EnvSettings{}
	if repo != "" {
		chartURL, err := helmRepo.FindChartInAuthAndTLSAndPassRepoURL(repo, "", "", chart, version, "", "", "", false, false, getter.All(settings))
		if err != nil {
			return nil, fmt.Errorf("unable to find chart: %w", err)
		}
		chart = chartURL
	}

	u, err := downloader.ResolveChartVersion(chart, version)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve chart version: %w", err)
	}

	getters := getter.All(settings)
	get, err := getters.ByScheme(u.Scheme)
	if err != nil {
		return nil, fmt.Errorf("unable to get getter for scheme %s: %w", u.Scheme, err)
	}

	b, err := get.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("unable to get chart: %w", err)
	}

	return b, nil
}
