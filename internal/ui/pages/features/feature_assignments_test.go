package features

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
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

	assert.Len(t, got, 2)
	assert.Equal(t, "DEPLOYED", got[0].StatusText)
	assert.Equal(t, "FAILED", got[1].StatusText)
}

func TestDeploymentDetailContentKeepsKebabOnlyInTableView(t *testing.T) {
	var buf bytes.Buffer
	node := assignmentDetailContent(&DetailPage{
		CurrentFeature: &model.Feature{Name: "naiserator"},
		AssignmentEnvs: []AssignmentEnvStatus{
			{Name: "dev", TenantName: "nav", TenantSlug: "nav", StatusText: "DEPLOYED", AssignmentVersion: "1.0.0"},
		},
	})
	assert.NoError(t, node.Render(&buf))

	html := buf.String()
	assert.Contains(t, html, `id="row-kebab-nav-dev"`)
	assert.Contains(t, html, `View logs`)
	assert.Contains(t, html, `/features/naiserator/envs/nav/dev`)
	assert.Equal(t, 1, strings.Count(html, `id="row-kebab-nav-dev"`))
	assert.Equal(t, 1, strings.Count(html, `View logs`))
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
	assert.NoError(t, node.Render(&buf))

	html := buf.String()
	// Full-width table for "all columns" mode
	assert.Contains(t, html, "overview-table")
	assert.Contains(t, html, "Tenant")
	assert.Contains(t, html, "atil")
	assert.Contains(t, html, "ci-nais")
	// Card grid for compact mode (one card per tenant)
	assert.Contains(t, html, "overview-grid")
	assert.Contains(t, html, "feature-card-header")
}

func TestStatusTooltip(t *testing.T) {
	t.Run("environment reconcile disabled with running version", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:              true,
			EnvReconcileDisabled: true,
			ReleaseVersion:       "1.2.3",
			AssignmentVersion:    "2.0.0",
			StatusText:           "DISABLED",
		}
		assert.Equal(t, "Environment reconcile disabled — Running: 1.2.3", statusTooltip(env))
	})

	t.Run("environment reconcile disabled without running version", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:              true,
			EnvReconcileDisabled: true,
			AssignmentVersion:    "2.0.0",
			StatusText:           "DISABLED",
		}
		assert.Equal(t, "Environment reconcile disabled", statusTooltip(env))
	})

	t.Run("feature reconcile disabled with running version", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           false,
			DisableReason:     "testing in progress",
			ReleaseVersion:    "1.0.0",
			AssignmentVersion: "2.0.0",
			StatusText:        "DISABLED",
		}
		assert.Equal(t, "Feature reconcile disabled: testing in progress — Running: 1.0.0", statusTooltip(env))
	})

	t.Run("feature reconcile disabled without running version", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           false,
			AssignmentVersion: "2.0.0",
			StatusText:        "DISABLED",
		}
		assert.Equal(t, "Feature reconcile disabled: disabled before we started requiring reason", statusTooltip(env))
	})

	t.Run("environment disabled takes priority over feature disabled", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:              false,
			EnvReconcileDisabled: true,
			ReleaseVersion:       "1.0.0",
			AssignmentVersion:    "2.0.0",
			StatusText:           "DISABLED",
		}
		assert.Equal(t, "Environment reconcile disabled — Running: 1.0.0", statusTooltip(env))
	})

	t.Run("overridden with version and target labels", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:             true,
			IsOverridden:        true,
			OverriddenByVersion: "3.0.0",
			OverriddenByLabels:  map[string]string{"tenant": "dev-nais", "kind": "tenant"},
			StatusText:          "OVERRIDDEN",
		}
		tip := statusTooltip(env)
		assert.Contains(t, tip, "Overridden by 3.0.0")
		assert.Contains(t, tip, "target:")
		assert.Contains(t, tip, "kind=tenant")
		assert.Contains(t, tip, "tenant=dev-nais")
	})

	t.Run("overridden without version", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:      true,
			IsOverridden: true,
			StatusText:   "OVERRIDDEN",
		}
		assert.Equal(t, "Overridden", statusTooltip(env))
	})

	t.Run("overridden takes priority over disabled", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:              false,
			EnvReconcileDisabled: true,
			IsOverridden:         true,
			OverriddenByVersion:  "3.0.0",
			StatusText:           "OVERRIDDEN",
		}
		assert.Contains(t, statusTooltip(env), "Overridden by 3.0.0")
	})

	t.Run("version drift shows currently running version", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "2.0.0",
			ReleaseVersion:    "1.5.0",
			StatusText:        "DEPLOYED",
		}
		assert.Equal(t, "Currently: 1.5.0", statusTooltip(env))
	})

	t.Run("deployed and in sync has no tooltip", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "2.0.0",
			ReleaseVersion:    "2.0.0",
			StatusText:        "DEPLOYED",
		}
		assert.Equal(t, "", statusTooltip(env))
	})

	t.Run("pending with no release shows not yet deployed", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "1.0.0",
			StatusText:        "PENDING",
		}
		assert.Equal(t, "No release deployed yet", statusTooltip(env))
	})

	t.Run("pending-install with no release shows not yet deployed", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "1.0.0",
			StatusText:        "PENDING-INSTALL",
		}
		assert.Equal(t, "No release deployed yet", statusTooltip(env))
	})

	t.Run("pending with release version shows drift instead", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "2.0.0",
			ReleaseVersion:    "1.0.0",
			StatusText:        "PENDING",
		}
		assert.Equal(t, "Currently: 1.0.0", statusTooltip(env))
	})

	t.Run("unknown status has no tooltip", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "1.0.0",
			StatusText:        "UNKNOWN",
		}
		assert.Equal(t, "", statusTooltip(env))
	})

	t.Run("failed status has no tooltip", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "1.0.0",
			ReleaseVersion:    "1.0.0",
			StatusText:        "FAILED",
		}
		assert.Equal(t, "", statusTooltip(env))
	})

	t.Run("created status treated as pending shows no-release tooltip", func(t *testing.T) {
		env := AssignmentEnvStatus{
			Enabled:           true,
			AssignmentVersion: "1.0.0",
			StatusText:        "PENDING",
		}
		assert.Equal(t, "No release deployed yet", statusTooltip(env))
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
