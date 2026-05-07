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
func EffectiveDeploymentStatus(ctx context.Context, repo database.Repo, envID uuid.UUID, featureName string, fallbackState string, fallbackModified time.Time) (status string, lastModified time.Time) {
	di, err := repo.DeployInstructionsLatestForFeature(ctx, envID, featureName)
	if err == nil && di != nil {
		return strings.ToUpper(di.Status.String()), di.LastModified
	}
	return fallbackState, fallbackModified
}
