package helm

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

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

	opts := []getter.Option{}
	if _, ok := os.LookupEnv("HTTPS_PROXY"); ok {
		opts = append(opts, getter.WithTransport(
			&http.Transport{
				// From https://github.com/google/go-containerregistry/blob/31786c6cbb82d6ec4fb8eb79cd9387905130534e/pkg/v1/remote/options.go#L87
				DisableCompression: true,
				Proxy:              http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					// By default we wrap the transport in retries, so reduce the
					// default dial timeout to 5s to avoid 5x 30s of connection
					// timeouts when doing the "ping" on certain http registries.
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}))
	}

	b, err := get.Get(u.String(), opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to get chart: %w", err)
	}

	return b, nil
}
