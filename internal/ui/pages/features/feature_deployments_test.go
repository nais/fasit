package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrentDeploymentEnvStatusesSkipsOverriddenRows(t *testing.T) {
	envs := []DeploymentEnvStatus{
		{
			Name:              "ci",
			TenantName:        "ci-nais",
			TenantSlug:        "ci-nais",
			DeploymentVersion: "1.0.0",
			StatusText:        "OVERRIDDEN",
			IsOverridden:      true,
		},
		{
			Name:              "ci",
			TenantName:        "ci-nais",
			TenantSlug:        "ci-nais",
			DeploymentVersion: "2.0.0",
			StatusText:        "DEPLOYED",
		},
		{
			Name:              "dev",
			TenantName:        "dev-nais",
			TenantSlug:        "dev-nais",
			DeploymentVersion: "1.0.0",
			StatusText:        "FAILED",
		},
	}

	got := currentDeploymentEnvStatuses(envs)

	assert.Len(t, got, 2)
	assert.Equal(t, "DEPLOYED", got[0].StatusText)
	assert.Equal(t, "FAILED", got[1].StatusText)
}

func TestStatusTooltip(t *testing.T) {
	t.Run("environment reconcile disabled with running version", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:              true,
			EnvReconcileDisabled: true,
			ReleaseVersion:       "1.2.3",
			DeploymentVersion:    "2.0.0",
			StatusText:           "DISABLED",
		}
		assert.Equal(t, "Environment reconcile disabled — Running: 1.2.3", statusTooltip(env))
	})

	t.Run("environment reconcile disabled without running version", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:              true,
			EnvReconcileDisabled: true,
			DeploymentVersion:    "2.0.0",
			StatusText:           "DISABLED",
		}
		assert.Equal(t, "Environment reconcile disabled", statusTooltip(env))
	})

	t.Run("feature reconcile disabled with running version", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           false,
			ReleaseVersion:    "1.0.0",
			DeploymentVersion: "2.0.0",
			StatusText:        "DISABLED",
		}
		assert.Equal(t, "Feature reconcile disabled — Running: 1.0.0", statusTooltip(env))
	})

	t.Run("feature reconcile disabled without running version", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           false,
			DeploymentVersion: "2.0.0",
			StatusText:        "DISABLED",
		}
		assert.Equal(t, "Feature reconcile disabled", statusTooltip(env))
	})

	t.Run("environment disabled takes priority over feature disabled", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:              false,
			EnvReconcileDisabled: true,
			ReleaseVersion:       "1.0.0",
			DeploymentVersion:    "2.0.0",
			StatusText:           "DISABLED",
		}
		assert.Equal(t, "Environment reconcile disabled — Running: 1.0.0", statusTooltip(env))
	})

	t.Run("overridden with version and target labels", func(t *testing.T) {
		env := DeploymentEnvStatus{
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
		env := DeploymentEnvStatus{
			Enabled:      true,
			IsOverridden: true,
			StatusText:   "OVERRIDDEN",
		}
		assert.Equal(t, "Overridden", statusTooltip(env))
	})

	t.Run("overridden takes priority over disabled", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:              false,
			EnvReconcileDisabled: true,
			IsOverridden:         true,
			OverriddenByVersion:  "3.0.0",
			StatusText:           "OVERRIDDEN",
		}
		assert.Contains(t, statusTooltip(env), "Overridden by 3.0.0")
	})

	t.Run("version drift shows currently running version", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "2.0.0",
			ReleaseVersion:    "1.5.0",
			StatusText:        "DEPLOYED",
		}
		assert.Equal(t, "Currently: 1.5.0", statusTooltip(env))
	})

	t.Run("deployed and in sync has no tooltip", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "2.0.0",
			ReleaseVersion:    "2.0.0",
			StatusText:        "DEPLOYED",
		}
		assert.Equal(t, "", statusTooltip(env))
	})

	t.Run("pending with no release shows not yet deployed", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "1.0.0",
			StatusText:        "PENDING",
		}
		assert.Equal(t, "No release deployed yet", statusTooltip(env))
	})

	t.Run("pending-install with no release shows not yet deployed", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "1.0.0",
			StatusText:        "PENDING-INSTALL",
		}
		assert.Equal(t, "No release deployed yet", statusTooltip(env))
	})

	t.Run("pending with release version shows drift instead", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "2.0.0",
			ReleaseVersion:    "1.0.0",
			StatusText:        "PENDING",
		}
		assert.Equal(t, "Currently: 1.0.0", statusTooltip(env))
	})

	t.Run("unknown status has no tooltip", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "1.0.0",
			StatusText:        "UNKNOWN",
		}
		assert.Equal(t, "", statusTooltip(env))
	})

	t.Run("failed status has no tooltip", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "1.0.0",
			ReleaseVersion:    "1.0.0",
			StatusText:        "FAILED",
		}
		assert.Equal(t, "", statusTooltip(env))
	})

	t.Run("created status treated as pending shows no-release tooltip", func(t *testing.T) {
		env := DeploymentEnvStatus{
			Enabled:           true,
			DeploymentVersion: "1.0.0",
			StatusText:        "PENDING",
		}
		assert.Equal(t, "No release deployed yet", statusTooltip(env))
	})
}
