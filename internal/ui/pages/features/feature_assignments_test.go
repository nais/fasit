package features

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
)

func TestCurrentAssignmentEnvStatusesSkipsOverriddenRows(t *testing.T) {
	envs := []AssignmentEnvStatus{
		{
			Name:              "ci",
			TenantName:        "ci-nais",
			TenantSlug:        "ci-nais",
			AssignmentVersion: "1.0.0",
			StatusText:        "OVERRIDDEN",
			IsOverridden:      true,
		},
		{
			Name:              "ci",
			TenantName:        "ci-nais",
			TenantSlug:        "ci-nais",
			AssignmentVersion: "2.0.0",
			StatusText:        "DEPLOYED",
		},
		{
			Name:              "dev",
			TenantName:        "dev-nais",
			TenantSlug:        "dev-nais",
			AssignmentVersion: "1.0.0",
			StatusText:        "FAILED",
		},
	}

	got := currentAssignmentEnvStatuses(envs)

	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].StatusText != "DEPLOYED" {
		t.Errorf("got[0].StatusText = %q, want DEPLOYED", got[0].StatusText)
	}
	if got[1].StatusText != "FAILED" {
		t.Errorf("got[1].StatusText = %q, want FAILED", got[1].StatusText)
	}
}

func TestDeploymentDetailContentKeepsKebabOnlyInTableView(t *testing.T) {
	var buf bytes.Buffer
	node := assignmentDetailContent(&DetailPage{
		CurrentFeature: &model.Feature{Name: "naiserator"},
		AssignmentEnvs: []AssignmentEnvStatus{
			{Name: "dev", TenantName: "nav", TenantSlug: "nav", StatusText: "DEPLOYED", AssignmentVersion: "1.0.0"},
		},
	})
	if err := node.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, `id="row-kebab-nav-dev"`) {
		t.Error("missing row-kebab-nav-dev id")
	}
	if !strings.Contains(html, `Deploy logs`) {
		t.Error("missing 'Deploy logs' text")
	}
	if !strings.Contains(html, `/features/naiserator/envs/nav/dev`) {
		t.Error("missing feature env link")
	}
	if c := strings.Count(html, `id="row-kebab-nav-dev"`); c != 1 {
		t.Errorf("row-kebab-nav-dev count = %d, want 1", c)
	}
	if c := strings.Count(html, `Deploy logs`); c != 1 {
		t.Errorf("Deploy logs count = %d, want 1", c)
	}
}

func TestDeploymentDetailContentRendersTableAndGrid(t *testing.T) {
	var buf bytes.Buffer
	node := assignmentDetailContent(&DetailPage{
		CurrentFeature: &model.Feature{Name: "naiserator"},
		AssignmentEnvs: []AssignmentEnvStatus{
			{Name: "dev", TenantName: "atil", TenantSlug: "atil", StatusText: "DEPLOYED", AssignmentVersion: "1.0.0"},
			{Name: "prod", TenantName: "atil", TenantSlug: "atil", StatusText: "DEPLOYED", AssignmentVersion: "1.0.0"},
			{Name: "ci", TenantName: "ci-nais", TenantSlug: "ci-nais", StatusText: "DEPLOYED", AssignmentVersion: "1.0.0"},
		},
	})
	if err := node.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	html := buf.String()
	for _, want := range []string{"overview-table", "Tenant", "atil", "ci-nais", "overview-grid", "feature-card-header"} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
}

func TestStatusTooltip(t *testing.T) {
	tests := []struct {
		name string
		env  AssignmentEnvStatus
		want string // exact match; use "" for contains-check tests
	}{
		{
			name: "environment reconcile disabled with running version",
			env: AssignmentEnvStatus{
				Enabled: true, EnvReconcileDisabled: true,
				ReleaseVersion: "1.2.3", AssignmentVersion: "2.0.0", StatusText: "DISABLED",
			},
			want: "Environment reconcile disabled — Running: 1.2.3",
		},
		{
			name: "environment reconcile disabled without running version",
			env: AssignmentEnvStatus{
				Enabled: true, EnvReconcileDisabled: true,
				AssignmentVersion: "2.0.0", StatusText: "DISABLED",
			},
			want: "Environment reconcile disabled",
		},
		{
			name: "feature reconcile disabled with running version",
			env: AssignmentEnvStatus{
				Enabled: false, DisableReason: "testing in progress",
				ReleaseVersion: "1.0.0", AssignmentVersion: "2.0.0", StatusText: "DISABLED",
			},
			want: "Feature reconcile disabled: testing in progress — Running: 1.0.0",
		},
		{
			name: "feature reconcile disabled without running version",
			env: AssignmentEnvStatus{
				Enabled: false, AssignmentVersion: "2.0.0", StatusText: "DISABLED",
			},
			want: "Feature reconcile disabled: disabled before we started requiring reason",
		},
		{
			name: "environment disabled takes priority over feature disabled",
			env: AssignmentEnvStatus{
				Enabled: false, EnvReconcileDisabled: true,
				ReleaseVersion: "1.0.0", AssignmentVersion: "2.0.0", StatusText: "DISABLED",
			},
			want: "Environment reconcile disabled — Running: 1.0.0",
		},
		{
			name: "overridden without version",
			env: AssignmentEnvStatus{
				Enabled: true, IsOverridden: true, StatusText: "OVERRIDDEN",
			},
			want: "Overridden",
		},
		{
			name: "version drift shows currently running version",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "2.0.0", ReleaseVersion: "1.5.0", StatusText: "DEPLOYED",
			},
			want: "Currently: 1.5.0",
		},
		{
			name: "deployed and in sync has no tooltip",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "2.0.0", ReleaseVersion: "2.0.0", StatusText: "DEPLOYED",
			},
			want: "",
		},
		{
			name: "pending with no release shows not yet deployed",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "1.0.0", StatusText: "PENDING",
			},
			want: "No release deployed yet",
		},
		{
			name: "pending-install with no release shows not yet deployed",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "1.0.0", StatusText: "PENDING-INSTALL",
			},
			want: "No release deployed yet",
		},
		{
			name: "pending with release version shows drift instead",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "2.0.0", ReleaseVersion: "1.0.0", StatusText: "PENDING",
			},
			want: "Currently: 1.0.0",
		},
		{
			name: "unknown status has no tooltip",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "1.0.0", StatusText: "UNKNOWN",
			},
			want: "",
		},
		{
			name: "failed status has no tooltip",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "1.0.0", ReleaseVersion: "1.0.0", StatusText: "FAILED",
			},
			want: "",
		},
		{
			name: "created status treated as pending shows no-release tooltip",
			env: AssignmentEnvStatus{
				Enabled: true, AssignmentVersion: "1.0.0", StatusText: "PENDING",
			},
			want: "No release deployed yet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := statusTooltip(tc.env)
			if got != tc.want {
				t.Errorf("statusTooltip() = %q, want %q", got, tc.want)
			}
		})
	}

	// Contains-check tests (tooltip has dynamic content)
	t.Run("overridden with version and target labels", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled: true, IsOverridden: true,
			OverriddenByVersion: "3.0.0",
			OverriddenByLabels:  map[string]string{"tenant": "dev-nais", "kind": "tenant"},
			StatusText:          "OVERRIDDEN",
		}
		tip := statusTooltip(env)
		for _, want := range []string{"Overridden by 3.0.0", "target:", "kind=tenant", "tenant=dev-nais"} {
			if !strings.Contains(tip, want) {
				t.Errorf("tooltip %q missing %q", tip, want)
			}
		}
	})

	t.Run("overridden takes priority over disabled", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled: false, EnvReconcileDisabled: true,
			IsOverridden: true, OverriddenByVersion: "3.0.0", StatusText: "OVERRIDDEN",
		}
		if !strings.Contains(statusTooltip(env), "Overridden by 3.0.0") {
			t.Errorf("tooltip = %q, want to contain 'Overridden by 3.0.0'", statusTooltip(env))
		}
	})
}

func TestFallbackVersionMap(t *testing.T) {
	t.Run("maps overrider to overridden version", func(t *testing.T) {
		envs := []AssignmentEnvStatus{
			{FeatureAssignmentID: "broad-1", AssignmentVersion: "1.0.0", IsOverridden: true, OverriddenByID: "specific-1"},
			{FeatureAssignmentID: "specific-1", AssignmentVersion: "2.0.0", IsOverridden: false},
		}
		fallbacks := fallbackVersionMap(envs)
		if got := fallbacks["specific-1"]; got != "1.0.0" {
			t.Errorf("fallbackVersionMap()[specific-1] = %q, want %q", got, "1.0.0")
		}
		if got := fallbacks["broad-1"]; got != "" {
			t.Errorf("fallbackVersionMap()[broad-1] = %q, want empty", got)
		}
	})

	t.Run("returns empty map when no overrides", func(t *testing.T) {
		envs := []AssignmentEnvStatus{
			{FeatureAssignmentID: "dep-1", AssignmentVersion: "1.0.0"},
			{FeatureAssignmentID: "dep-2", AssignmentVersion: "2.0.0"},
		}
		fallbacks := fallbackVersionMap(envs)
		if len(fallbacks) != 0 {
			t.Errorf("fallbackVersionMap() = %v, want empty map", fallbacks)
		}
	})
}
