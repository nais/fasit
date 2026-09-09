package helm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/helm"
)

func TestListChartVersions(t *testing.T) {
	registryServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/charts/example/tags/list":
			if r.URL.Query().Get("last") == "" {
				w.Header().Set("Link", fmt.Sprintf(`<%s/v2/charts/example/tags/list?last=1.2.3_build.1>; rel="next"`, "https://"+r.Host))
				writeTagsList(t, w, map[string]any{
					"name": "charts/example",
					"tags": []string{"1.0.0", "1.2.3_build.1"},
					"manifest": map[string]any{
						"sha256:a": map[string]any{"tag": []string{"1.0.0"}, "timeUploadedMs": "1000"},
						"sha256:b": map[string]any{"tag": []string{"1.2.3_build.1"}, "timeUploadedMs": "2000"},
					},
				})
				return
			}
			writeTagsList(t, w, map[string]any{
				"name": "charts/example",
				"tags": []string{"latest", "2.1.0", "2026-09-09-095744-e41bab5"},
				"manifest": map[string]any{
					"sha256:c": map[string]any{"tag": []string{"2.1.0"}, "timeUploadedMs": "3000"},
					"sha256:d": map[string]any{"tag": []string{"2026-09-09-095744-e41bab5"}, "timeUploadedMs": "4000"},
				},
			})
		default:
			http.NotFound(w, r)
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

	// Sorted by upload time across pages; "latest" has no upload time and sorts last.
	want := []string{"2026-09-09-095744-e41bab5", "2.1.0", "1.2.3_build.1", "1.0.0", "latest"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListChartVersions(%q) = %v, want %v", chartRef, got, want)
	}
}

func writeTagsList(t *testing.T, w http.ResponseWriter, body map[string]any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encode registry response: %v", err)
	}
}
