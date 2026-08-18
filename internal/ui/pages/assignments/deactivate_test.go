package assignments

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
			name:     "no referer redirects to assignments",
			expected: "/assignments",
		},
		{
			name:     "explicit redirect field takes priority",
			formBody: "redirect=/features/naiserator/assignments",
			referer:  "http://localhost:8080/assignments",
			expected: "/features/naiserator/assignments",
		},
		{
			name:     "referer from feature page redirects back",
			referer:  "http://localhost:8080/features/naiserator/assignments",
			expected: "/features/naiserator/assignments",
		},
		{
			name:     "referer from assignments page redirects to assignments",
			referer:  "http://localhost:8080/assignments",
			expected: "/assignments",
		},
		{
			name:     "referer from external URL redirects to assignments",
			referer:  "http://other.example.com/features/foo",
			expected: "/assignments",
		},
		{
			name:     "redirect field must start with slash",
			formBody: "redirect=http://evil.com/steal",
			expected: "/assignments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.formBody != "" {
				body = strings.NewReader(tt.formBody)
			}
			r, _ := http.NewRequest("POST", "http://localhost:8080/assignments/123/remove", body)
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
