package view

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
)

// EffectiveDeploymentStatus returns the status for a feature in an environment,
// preferring the deploy instruction status (updated by the receiver when naisd
// responds) over deployment_statuses (written once by the reconciler).
//
// When the upstream status reports DEPLOYED but the actual release version
// differs from the desired deployment version, the status is downgraded to
// PENDING to surface drift / not-yet-converged state.
func EffectiveDeploymentStatus(ctx context.Context, repo database.Repo, envID uuid.UUID, featureName string, fallbackState string, fallbackModified time.Time, desiredVersion, releaseVersion string) (status string, lastModified time.Time) {
	status = fallbackState
	lastModified = fallbackModified
	if di, err := repo.DeployInstructionsLatestForFeature(ctx, envID, featureName); err == nil && di != nil {
		status = strings.ToUpper(di.Status.String())
		lastModified = di.LastModified
	}
	if status == "DEPLOYED" && releaseVersion != "" && desiredVersion != "" && releaseVersion != desiredVersion {
		status = "PENDING"
	}
	return status, lastModified
}
