package helm

import (
	"context"
	"testing"

	"golang.org/x/oauth2"
)

func TestIsGoogleArtifactRegistry(t *testing.T) {
	tests := []struct {
		chartRef string
		want     bool
	}{
		{chartRef: "oci://europe-north1-docker.pkg.dev/nais-io/nais/charts/example", want: true},
		{chartRef: "oci://registry.example.com/charts/example", want: false},
		{chartRef: "not a URL", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.chartRef, func(t *testing.T) {
			if got := isGoogleArtifactRegistry(tt.chartRef); got != tt.want {
				t.Errorf("isGoogleArtifactRegistry(%q) = %t, want %t", tt.chartRef, got, tt.want)
			}
		})
	}
}

func TestGoogleArtifactRegistryCredential(t *testing.T) {
	credential := googleArtifactRegistryCredential(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "access-token"}))

	got, err := credential(context.Background(), "europe-north1-docker.pkg.dev")
	if err != nil {
		t.Fatalf("credential() returned an error: %v", err)
	}
	if got.AccessToken != "access-token" {
		t.Errorf("credential().AccessToken = %q, want %q", got.AccessToken, "access-token")
	}
}
