package helm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/registry"
)

const registryHTTPTimeout = 3 * time.Second

type contextTransport struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(req.WithContext(t.ctx))
}

// ListChartVersions returns the semver-compliant tags for an OCI chart, newest first.
func ListChartVersions(ctx context.Context, chartRef string) ([]string, error) {
	httpClient := &http.Client{
		Timeout:   registryHTTPTimeout,
		Transport: contextTransport{ctx: ctx, base: http.DefaultTransport},
	}
	client, err := registry.NewClient(registry.ClientOptHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create registry client for OCI chart %q: %w", chartRef, err)
	}

	versions, err := client.Tags(strings.TrimPrefix(chartRef, registry.OCIScheme+"://"))
	if err != nil {
		return nil, fmt.Errorf("list versions for OCI chart %q: %w", chartRef, err)
	}

	return versions, nil
}
