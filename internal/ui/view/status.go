package view

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
)

// EnvironmentStatusCounts returns the number of features in this environment
// whose latest deploy instruction is failed or pending. Uses the same data
// sources as the GraphQL FeatureStates resolver (ListEnvironmentFeatures +
// FeatureStatesGet) so rollout-only features are included.
func EnvironmentStatusCounts(ctx context.Context, repo database.Repo, envID uuid.UUID) (failed, pending int) {
	deploymentFeatures, err := deployment.ListEnvironmentFeatures(ctx, envID)
	if err != nil {
		return 0, 0
	}
	seen := make(map[string]bool, len(deploymentFeatures))
	for _, f := range deploymentFeatures {
		seen[f.FeatureName] = true
	}
	states, err := featurepkg.FeatureStatesGet(ctx, envID)
	if err != nil {
		return 0, 0
	}
	for _, state := range states {
		if !seen[state.FeatureName] {
			seen[state.FeatureName] = true
		}
	}
	for name := range seen {
		f, p := FeatureStatusForEnv(ctx, repo, envID, name)
		if f {
			failed++
		} else if p {
			pending++
		}
	}
	return failed, pending
}

// FeatureStatusForEnv reports whether the latest deploy instruction for
// (environment, feature) is failed or pending. Deploy instructions are the
// unified source of truth for both rollout-driven and deployment-driven
// progress, so this single lookup covers both paths.
func FeatureStatusForEnv(ctx context.Context, repo database.Repo, envID uuid.UUID, featureName string) (failed, pending bool) {
	di, err := repo.DeployInstructionsLatestForFeature(ctx, envID, featureName)
	if err != nil || di == nil {
		return false, false
	}
	switch di.Status {
	case model.RolloutStatusFailed:
		return true, false
	case model.RolloutStatusPending, model.RolloutStatusCreated:
		return false, true
	}
	return false, false
}
