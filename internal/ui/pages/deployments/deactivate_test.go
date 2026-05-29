package deployments

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDeactivateRedirect(t *testing.T) {
	tests := []struct {
		name     string
		referer  string
		formBody string
		expected string
	}{
		{
			name:     "no referer redirects to deployments",
			expected: "/deployments",
		},
		{
			name:     "explicit redirect field takes priority",
			formBody: "redirect=/features/naiserator/deploy-specs",
			referer:  "http://localhost:8080/deployments",
			expected: "/features/naiserator/deploy-specs",
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
		{
			name:     "redirect field must start with slash",
			formBody: "redirect=http://evil.com/steal",
			expected: "/deployments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.formBody != "" {
				body = strings.NewReader(tt.formBody)
			}
			r, _ := http.NewRequest("POST", "http://localhost:8080/deployments/123/deactivate", body)
			if tt.formBody != "" {
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
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
