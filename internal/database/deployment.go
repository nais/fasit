package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/environment"
)

type DeploymentRepo interface {
	DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target environment.Labels, hash string) (*gensql.Deployment, error)
	DeploymentTargetsGetAll(ctx context.Context) ([]gensql.DeploymentTargetsGetAllRow, error)
	DeploymentTargetsGet(ctx context.Context, deploymentID uuid.UUID) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
	DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]gensql.DeploymentsForEnvironmentRow, error)
	FeatureEnabled(ctx context.Context, featureName string, envID uuid.UUID) (bool, error)
	DeployInstructionsGetFeaturesNotInEnv(ctx context.Context, features []string, environmentID uuid.UUID) ([]string, error)
}

func (r *repo) DeployInstructionsGetFeaturesNotInEnv(ctx context.Context, features []string, environmentID uuid.UUID) ([]string, error) {
	if len(features) == 0 {
		return []string{}, nil
	}
	return r.querier.DeployInstructionsGetFeaturesNotInEnv(ctx, gensql.DeployInstructionsGetFeaturesNotInEnvParams{
		FeatureNames:  features,
		EnvironmentID: environmentID,
	})
}

func (r *repo) DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]gensql.DeploymentsForEnvironmentRow, error) {
	return r.querier.DeploymentsForEnvironment(ctx, environmentID)
}

func (r *repo) DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target environment.Labels, hash string) (*gensql.Deployment, error) {
	ret, err := r.querier.DeploymentCreate(ctx, gensql.DeploymentCreateParams{
		FeatureName: featureName,
		Version:     featureVersion,
		GhRef:       ghRef,
		Target:      target,
		Hash:        hash,
	})
	return &ret, err
}

func (r *repo) DeploymentTargetsGetAll(ctx context.Context) ([]gensql.DeploymentTargetsGetAllRow, error) {
	return r.querier.DeploymentTargetsGetAll(ctx)
}

func (r *repo) DeploymentTargetsGet(ctx context.Context, deploymentID uuid.UUID) ([]gensql.DeploymentTarget, error) {
	return r.querier.DeploymentTargetsGet(ctx, deploymentID)
}

func (r *repo) DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error) {
	return r.querier.DeploymentTargetsGetPending(ctx)
}

func (r *repo) DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error) {
	return r.querier.DeploymentsGet(ctx)
}

func (r *repo) DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error {
	return r.querier.DeploymentTargetsCreate(ctx, gensql.DeploymentTargetsCreateParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
	})
}

func (r *repo) DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error {
	return r.querier.DeploymentTargetsUpdate(ctx, gensql.DeploymentTargetsUpdateParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
		Status:        status,
	})
}

func (r *repo) FeatureEnabled(ctx context.Context, featureName string, envID uuid.UUID) (bool, error) {
	return r.querier.FeatureEnabled(ctx, gensql.FeatureEnabledParams{
		FeatureName:   featureName,
		EnvironmentID: envID,
	})
}
