package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nsf/jsondiff"
)

type DeployInstructionRepo interface {
	DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error)
	TimeoutDeployInstructions(ctx context.Context)
	DeployInstructionsForFeature(ctx context.Context, envID uuid.UUID, featureName string, offset int) ([]*model.DeployInstruction, error)
	DeployInstructionsLatestForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error)
	DeployInstructionUpdateStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error
	HelmValueDiffGet(ctx context.Context, di *model.DeployInstruction) (*model.HelmValueDiff, error)
	NamesFromDeployInstruction(ctx context.Context, id uuid.UUID) (tenantName, environmentName string, err error)
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
		Offset:        int32(offset), // #nosec G115
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

func (r *repo) TimeoutDeployInstructions(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		err := r.querier.TimeoutDeployInstructions(ctx)
		if err != nil {
			r.log.WithError(err).Error("failed to timeout deploy instructions")
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *repo) HelmValueDiffGet(ctx context.Context, di *model.DeployInstruction) (*model.HelmValueDiff, error) {
	ret := &model.HelmValueDiff{
		Difference: model.HelmValueDifferenceNoMatch,
	}

	prev, err := r.querier.DeployInstructionsPrevious(ctx, di.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ret, nil
		}
		return nil, fmt.Errorf("failed to get previous deploy instruction: %w", err)
	}

	opts := jsondiff.DefaultHTMLOptions()
	opts.Indent = "\t"
	opts.PrintTypes = true
	opts.SkipMatches = true
	diff, diff2 := jsondiff.Compare(prev.Values, di.Values, &opts)
	ret.Diff = diff2

	switch diff {
	case jsondiff.FullMatch:
		ret.Difference = model.HelmValueDifferenceFullMatch
	case jsondiff.SupersetMatch:
		ret.Difference = model.HelmValueDifferenceSupersetMatch
	case jsondiff.BothArgsAreInvalidJson, jsondiff.FirstArgIsInvalidJson, jsondiff.SecondArgIsInvalidJson:
		ret.Difference = model.HelmValueDifferenceInvalidJSON
	}

	return ret, nil
}

func (r *repo) NamesFromDeployInstruction(ctx context.Context, id uuid.UUID) (tenantName, environmentName string, err error) {
	row, err := r.querier.NamesFromDeployInstruction(ctx, id)
	if err != nil {
		return "", "", err
	}

	return row.TenantName, row.EnvironmentName, nil
}

func deployInstructionFromSQL(di gensql.DeployInstruction) *model.DeployInstruction {
	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		DeploymentID:   di.DeploymentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Status:         model.RolloutStatus(di.Status),
		Hash:           di.Hash,
		Created:        di.Created.Time,
		LastModified:   di.LastModified.Time,
		Values:         di.Values,
	}
}
