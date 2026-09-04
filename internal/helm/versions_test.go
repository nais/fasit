package helm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/helm"
)

func TestListChartVersions(t *testing.T) {
	registryServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/charts/example/tags/list" {
			http.NotFound(w, r)
			return
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"name": "charts/example",
			"tags": []string{"1.0.0", "latest", "2.1.0", "1.2.3_build.1"},
		}); err != nil {
			t.Errorf("encode registry response: %v", err)
		}
	}))
	defer registryServer.Close()

	previousTransport := http.DefaultTransport
	http.DefaultTransport = registryServer.Client().Transport
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
	})

	chartRef := "oci://" + strings.TrimPrefix(registryServer.URL, "https://") + "/charts/example"
	got, err := helm.ListChartVersions(context.Background(), chartRef)
	if err != nil {
		t.Fatalf("ListChartVersions(%q) returned an error: %v", chartRef, err)
	}

	want := []string{"2.1.0", "1.2.3+build.1", "1.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListChartVersions(%q) = %v, want %v", chartRef, got, want)
	}
}
