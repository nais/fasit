package features

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nais/fasit/internal/audit"
)

func TestComputeAggStatus(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []string
		wantClass string
		wantLabel string
	}{
		{
			name:      "all deployed",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "DEPLOYED"},
			wantClass: "status-success",
			wantLabel: "DEPLOYED",
		},
		{
			name:      "single deployed",
			statuses:  []string{"DEPLOYED"},
			wantClass: "status-success",
			wantLabel: "DEPLOYED",
		},
		{
			name:      "in progress no failures",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "PENDING"},
			wantClass: "status-pending",
			wantLabel: "2/3 deployed",
		},
		{
			name:      "all pending",
			statuses:  []string{"PENDING", "PENDING", "PENDING"},
			wantClass: "status-pending",
			wantLabel: "0/3 deployed",
		},
		{
			name:      "mixed with failures",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "FAILED", "PENDING"},
			wantClass: "status-error",
			wantLabel: "2 deployed, 1 failed, 1 pending",
		},
		{
			name:      "some failed rest deployed",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "DEPLOYED", "FAILED"},
			wantClass: "status-error",
			wantLabel: "3 deployed, 1 failed",
		},
		{
			name:      "all failed",
			statuses:  []string{"FAILED", "FAILED", "FAILED"},
			wantClass: "status-error",
			wantLabel: "3 failed",
		},
		{
			name:      "pending install variant",
			statuses:  []string{"DEPLOYED", "PENDING-INSTALL"},
			wantClass: "status-pending",
			wantLabel: "1/2 deployed",
		},
		{
			name:      "unknown statuses counted",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "SOMETHING_ELSE"},
			wantClass: "status-pending",
			wantLabel: "2/3 deployed",
		},
		{
			name:      "disabled counts as deployed",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "DISABLED"},
			wantClass: "status-success",
			wantLabel: "DEPLOYED",
		},
		{
			name:      "realistic: 15 deployed 2 failed 1 pending",
			statuses:  repeat("DEPLOYED", 15, "FAILED", 2, "PENDING", 1),
			wantClass: "status-error",
			wantLabel: "15 deployed, 2 failed, 1 pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeAggStatus(tt.statuses)
			if got.class != tt.wantClass {
				t.Errorf("class: got %q, want %q", got.class, tt.wantClass)
			}
			if got.label != tt.wantLabel {
				t.Errorf("label: got %q, want %q", got.label, tt.wantLabel)
			}
		})
	}
}

func TestDeploymentActorsByID(t *testing.T) {
	actors := deploymentActorsByID([]*audit.Entry{
		{
			Actor:      "tronghn@nais/fasit/123456789",
			Action:     audit.ActionCreated,
			ObjectType: audit.ObjectTypeDeployment,
			Metadata:   []byte(`{"deploymentId":"dep-1"}`),
		},
		{
			Actor:      "ignored@nais/fasit/1",
			Action:     audit.ActionUpdated,
			ObjectType: audit.ObjectTypeDeployment,
			Metadata:   []byte(`{"deploymentId":"dep-2"}`),
		},
	})

	if got := actors["dep-1"]; got != "tronghn@nais/fasit/123456789" {
		t.Fatalf("got %q, want deployment actor", got)
	}
	if _, ok := actors["dep-2"]; ok {
		t.Fatal("non-created deployment audit should not be used as deployment actor")
	}
}

func TestRecentDeploymentsRendersActor(t *testing.T) {
	var buf bytes.Buffer
	err := recentDeployments([]depRow{{
		FeatureName: "kyverno",
		Version:     "2026-05-27-abc",
		Status:      "DEPLOYED",
		Created:     time.Now(),
		DepID:       "dep-1",
		Actor:       "tronghn@nais/fasit/123456789",
	}}).Render(&buf)
	if err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	if !strings.Contains(html, "@tronghn via ") || !strings.Contains(html, "github.com/nais/fasit/actions/runs/123456789") {
		t.Fatalf("recent deployments should render workflow actor, got %s", html)
	}
}

func repeat(args ...any) []string {
	var out []string
	for i := 0; i < len(args); i += 2 {
		s := args[i].(string)
		n := args[i+1].(int)
		for range n {
			out = append(out, s)
		}
	}
	return out
}
