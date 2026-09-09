package helm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"helm.sh/helm/v3/pkg/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	registryHTTPTimeout = 15 * time.Second
	cloudPlatformScope  = "https://www.googleapis.com/auth/cloud-platform"
)

// ListChartVersions returns the semver-compliant tags for an OCI chart, newest first.
func ListChartVersions(ctx context.Context, chartRef string) ([]string, error) {
	httpClient := &http.Client{Timeout: registryHTTPTimeout}
	options := []registry.ClientOption{registry.ClientOptHTTPClient(httpClient)}
	if isGoogleArtifactRegistry(chartRef) {
		credentials, err := google.FindDefaultCredentials(ctx, cloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("find Google credentials for OCI chart %q: %w", chartRef, err)
		}
		options = append(options, registry.ClientOptAuthorizer(auth.Client{
			Client:     httpClient,
			Credential: googleArtifactRegistryCredential(credentials.TokenSource),
		}))
	}
	client, err := registry.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create registry client for OCI chart %q: %w", chartRef, err)
	}

	versions, err := client.Tags(strings.TrimPrefix(chartRef, registry.OCIScheme+"://"))
	if err != nil {
		return nil, fmt.Errorf("list versions for OCI chart %q: %w", chartRef, err)
	}

	return versions, nil
}

func isGoogleArtifactRegistry(chartRef string) bool {
	ref, err := url.Parse(chartRef)
	return err == nil && strings.HasSuffix(ref.Hostname(), ".pkg.dev")
}

func googleArtifactRegistryCredential(tokenSource oauth2.TokenSource) auth.CredentialFunc {
	return func(ctx context.Context, _ string) (auth.Credential, error) {
		token, err := tokenSource.Token()
		if err != nil {
			return auth.EmptyCredential, fmt.Errorf("get Google access token: %w", err)
		}
		return auth.Credential{AccessToken: token.AccessToken}, nil
	}
}
