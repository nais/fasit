package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type DeployInstructionRepo interface {
	DeployInstructionCreate(ctx context.Context, envID uuid.UUID, featureName, featureVersion, hash string) (uuid.UUID, error)
	DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error)
}

func (r *repo) DeployInstructionCreate(ctx context.Context, envID uuid.UUID, featureName, featureVersion, hash string) (uuid.UUID, error) {
	return r.querier.DeployInstructionsCreate(ctx, gensql.DeployInstructionsCreateParams{
		EnvironmentID:  envID,
		FeatureName:    featureName,
		FeatureVersion: featureVersion,
		Hash:           hash,
	})
}

func (r *repo) DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error) {
	di, err := r.querier.DeployInstructionsByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Hash:           di.Hash,
	}, nil
}
