package database

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
)

type (
	EnvironmentLabels     = map[EnvironmentLabelKey]EnvironmentLabelValue
	EnvironmentID         = uuid.UUID
	EnvironmentLabelKey   = string
	EnvironmentLabelValue = string
)

type DeploymentRepo interface {
	DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target EnvironmentLabels) error
	DeploymentTargetsGet(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
}

func (r *repo) DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target EnvironmentLabels) error {
	b, err := json.Marshal(target)
	if err != nil {
		return err
	}
	return r.querier.DeploymentCreate(ctx, gensql.DeploymentCreateParams{
		FeatureName: featureName,
		Version:     featureVersion,
		GhRef:       ghRef,
		Target:      b,
	})
}

func (r *repo) DeploymentTargetsGet(ctx context.Context) ([]gensql.DeploymentTarget, error) {
	return r.querier.DeploymentTargetsGet(ctx)
}

func (r *repo) DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error) {
	return r.querier.DeploymentTargetsGet(ctx)
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
