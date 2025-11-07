package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/environment"
)

type DeploymentRepo interface {
	DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target environment.Labels) (*gensql.Deployment, error)
	DeploymentTargetsGetAll(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGet(ctx context.Context, deploymentID uuid.UUID) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
	DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]gensql.Deployment, error)
	FeatureEnabled(ctx context.Context, featureName string, envID uuid.UUID) (bool, error)
}

func (r *repo) DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]gensql.Deployment, error) {
	return r.querier.DeploymentsForEnvironment(ctx, environmentID)
}

func (r *repo) DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target environment.Labels) (*gensql.Deployment, error) {
	ret, err := r.querier.DeploymentCreate(ctx, gensql.DeploymentCreateParams{
		FeatureName: featureName,
		Version:     featureVersion,
		GhRef:       ghRef,
		Target:      target,
	})
	return &ret, err
}

func (r *repo) DeploymentTargetsGetAll(ctx context.Context) ([]gensql.DeploymentTarget, error) {
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
		Hash:          "",
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
