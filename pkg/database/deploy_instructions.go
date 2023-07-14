package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type DeployInstructionRepo interface {
	DeployInstructionCreate(ctx context.Context, envID uuid.UUID, featureName, featureVersion, hash string) (uuid.UUID, error)
	DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error)
	DeployInstructionsForFeature(ctx context.Context, envID uuid.UUID, featureName string, offset int) ([]*model.DeployInstruction, error)
	DeployInstructionsLatestForEnvironment(ctx context.Context, envID uuid.UUID) ([]*model.DeployInstruction, error)
	DeployInstructionsLatestForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error)
	DeployInstructionUpdateStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error
}

func (r *repo) DeployInstructionCreate(ctx context.Context, envID uuid.UUID, featureName, featureVersion, hash string) (uuid.UUID, error) {
	return r.querier.DeployInstructionsCreate(ctx, gensql.DeployInstructionsCreateParams{
		EnvironmentID:  envID,
		FeatureName:    featureName,
		FeatureVersion: featureVersion,
		Hash:           hash,
	})
}

func (r *repo) DeployInstructionUpdateStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %q", status)
	}
	return r.querier.DeployInstructionsUpdateStatus(ctx, gensql.DeployInstructionsUpdateStatusParams{
		ID:     id,
		Status: status.String(),
	})
}

func (r *repo) DeployInstructionsForFeature(ctx context.Context, envID uuid.UUID, featureName string, offset int) ([]*model.DeployInstruction, error) {
	dis, err := r.querier.DeployInstructionsForFeature(ctx, gensql.DeployInstructionsForFeatureParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
		Offset:        int32(offset),
	})
	if err != nil {
		return nil, err
	}

	instructions := make([]*model.DeployInstruction, len(dis))
	for i, di := range dis {
		instructions[i] = deployInstructionFromSQL(di)
	}

	return instructions, nil
}

func (r *repo) DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error) {
	di, err := r.querier.DeployInstructionsByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return deployInstructionFromSQL(di), nil
}

func (r *repo) DeployInstructionsLatestForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error) {
	di, err := r.querier.DeployInstructionsLatestForFeature(ctx, gensql.DeployInstructionsLatestForFeatureParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	return deployInstructionFromSQL(di), nil
}

func (r *repo) DeployInstructionsLatestForEnvironment(ctx context.Context, envID uuid.UUID) ([]*model.DeployInstruction, error) {
	dis, err := r.querier.DeployInstructionsLatestForEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}

	var instructions []*model.DeployInstruction
	for _, di := range dis {
		instructions = append(instructions, deployInstructionFromSQL(di))
	}

	return instructions, nil
}

func deployInstructionFromSQL(di gensql.DeployInstruction) *model.DeployInstruction {
	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Status:         model.RolloutStatus(di.Status),
		Hash:           di.Hash,
		Created:        di.Created.Time,
		LastModified:   di.LastModified.Time,
	}
}
