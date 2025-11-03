package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
)

type DeploymentRepo interface {
	DeploymentTargetsGet(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
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
