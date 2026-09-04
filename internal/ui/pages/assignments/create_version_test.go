package assignments

import (
	"net/url"
	"testing"
)

func TestRequestedVersion(t *testing.T) {
	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{name: "selected version", form: url.Values{"version": {"2.0.0"}}, want: "2.0.0"},
		{name: "custom version", form: url.Values{"version": {"__custom__"}, "version_custom": {" 2.1.0-rc.1 "}}, want: "2.1.0-rc.1"},
		{name: "missing custom version", form: url.Values{"version": {"__custom__"}}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := requestedVersion(tc.form); got != tc.want {
				t.Errorf("requestedVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}
