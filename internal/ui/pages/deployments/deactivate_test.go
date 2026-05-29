package deployments

import (
	"net/http"
	"testing"
)

func TestDeactivateRedirect(t *testing.T) {
	tests := []struct {
		name     string
		referer  string
		expected string
	}{
		{
			name:     "no referer redirects to deployments",
			referer:  "",
			expected: "/deployments",
		},
		{
			name:     "referer from feature page redirects back",
			referer:  "http://localhost:8080/features/naiserator/deploy-specs",
			expected: "/features/naiserator/deploy-specs",
		},
		{
			name:     "referer from deployments page redirects to deployments",
			referer:  "http://localhost:8080/deployments",
			expected: "/deployments",
		},
		{
			name:     "referer from external URL redirects to deployments",
			referer:  "http://other.example.com/features/foo",
			expected: "/deployments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("POST", "http://localhost:8080/deployments/123/deactivate", nil)
			if tt.referer != "" {
				r.Header.Set("Referer", tt.referer)
			}
			got := deactivateRedirect(r)
			if got != tt.expected {
				t.Errorf("deactivateRedirect() = %q, want %q", got, tt.expected)
			}
		})
	}
}
