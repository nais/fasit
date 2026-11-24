package daemon

import (
	"testing"
	"time"
)

func TestCapSelfUpgradeTimeout(t *testing.T) {
	tests := map[string]struct {
		timeout time.Duration
		grace   time.Duration
		want    time.Duration
	}{
		"no grace period configured": {timeout: 20 * time.Minute, grace: 0, want: 20 * time.Minute},
		"fits inside grace period":   {timeout: 5 * time.Minute, grace: 10 * time.Minute, want: 5 * time.Minute},
		"capped to grace minus margin": {
			timeout: 20 * time.Minute, grace: 10 * time.Minute, want: 10*time.Minute - selfUpgradeMargin,
		},
		"unset timeout uses helm default then caps": {
			timeout: 0, grace: 3 * time.Minute, want: 3*time.Minute - selfUpgradeMargin,
		},
		"grace shorter than margin disables cap": {timeout: 5 * time.Minute, grace: 30 * time.Second, want: 5 * time.Minute},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := capSelfUpgradeTimeout(tc.timeout, tc.grace); got != tc.want {
				t.Errorf("capSelfUpgradeTimeout(%v, %v) = %v, want %v", tc.timeout, tc.grace, got, tc.want)
			}
		})
	}
}
