package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	registryHTTPTimeout = 15 * time.Second
	cloudPlatformScope  = "https://www.googleapis.com/auth/cloud-platform"
)

// ListChartVersions returns all tags for an OCI chart, newest upload first.
// Upload time comes from the registry's tags/list manifest metadata (a GCR/GAR
// extension); tags without a known upload time sort last, newest tag name first.
func ListChartVersions(ctx context.Context, chartRef string) ([]string, error) {
	authClient := &auth.Client{Client: &http.Client{Timeout: registryHTTPTimeout}}
	if isGoogleArtifactRegistry(chartRef) {
		credentials, err := google.FindDefaultCredentials(ctx, cloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("find Google credentials for OCI chart %q: %w", chartRef, err)
		}
		authClient.Credential = googleArtifactRegistryCredential(credentials.TokenSource)
	}

	host, repoPath, ok := strings.Cut(strings.TrimPrefix(chartRef, "oci://"), "/")
	if !ok || host == "" || repoPath == "" {
		return nil, fmt.Errorf("invalid OCI chart reference %q", chartRef)
	}

	tags, uploaded, err := listTags(ctx, authClient, host, repoPath)
	if err != nil {
		return nil, fmt.Errorf("list versions for OCI chart %q: %w", chartRef, err)
	}

	sort.SliceStable(tags, func(i, j int) bool {
		if ti, tj := uploaded[tags[i]], uploaded[tags[j]]; ti != tj {
			return ti > tj
		}
		return tags[i] > tags[j]
	})
	return tags, nil
}

type tagsListResponse struct {
	Tags      []string `json:"tags"`
	Manifests map[string]struct {
		Tags           []string `json:"tag"`
		UploadedMillis string   `json:"timeUploadedMs"`
	} `json:"manifest"`
}

func listTags(ctx context.Context, client *auth.Client, host, repoPath string) ([]string, map[string]int64, error) {
	var tags []string
	uploaded := make(map[string]int64)
	nextURL := "https://" + host + "/v2/" + repoPath + "/tags/list"
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		var page tagsListResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&page)
		closeErr := resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("registry returned %s", resp.Status)
		}
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("decode tags list: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, nil, fmt.Errorf("close tags list response: %w", closeErr)
		}
		tags = append(tags, page.Tags...)
		for _, manifest := range page.Manifests {
			millis, err := strconv.ParseInt(manifest.UploadedMillis, 10, 64)
			if err != nil {
				continue
			}
			for _, tag := range manifest.Tags {
				uploaded[tag] = millis
			}
		}
		nextURL = nextTagsPageURL(resp.Header.Get("Link"), resp.Request.URL)
	}
	return tags, uploaded, nil
}

func nextTagsPageURL(linkHeader string, base *url.URL) string {
	for link := range strings.SplitSeq(linkHeader, ",") {
		partA, partB, ok := strings.Cut(link, ";")
		if !ok || strings.TrimSpace(partB) != `rel="next"` {
			continue
		}
		ref, err := url.Parse(strings.Trim(strings.TrimSpace(partA), "<>"))
		if err != nil {
			return ""
		}
		return base.ResolveReference(ref).String()
	}
	return ""
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
