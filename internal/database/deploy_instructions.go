package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nsf/jsondiff"
)

type DeployInstructionRepo interface {
	DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error)
	DeployInstructionsForFeature(ctx context.Context, envID uuid.UUID, featureName string, offset int) ([]*model.DeployInstruction, error)
	DeployInstructionsLatestForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error)
	DeployInstructionsLatestDeployedForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error)
	DeployInstructionStatusCounts(ctx context.Context) (failed, pending map[string]int, err error)
	DeployInstructionUpdateStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error
	HelmValueDiffGet(ctx context.Context, di *model.DeployInstruction, secretKeys []string) (*model.HelmValueDiff, error)
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

func (r *repo) DeployInstructionsLatestDeployedForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error) {
	di, err := r.querier.DeployInstructionsLatestDeployedForFeature(ctx, gensql.DeployInstructionsLatestDeployedForFeatureParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return deployInstructionFromSQL(di), nil
}

func (r *repo) DeployInstructionStatusCounts(ctx context.Context) (failed, pending map[string]int, err error) {
	failed = map[string]int{}
	pending = map[string]int{}
	rows, err := r.querier.DeployInstructionStatusCounts(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		if row.FailedCount > 0 {
			failed[row.FeatureName] = int(row.FailedCount)
		}
		if row.PendingCount > 0 {
			pending[row.FeatureName] = int(row.PendingCount)
		}
	}
	return failed, pending, nil
}

func (r *repo) HelmValueDiffGet(ctx context.Context, di *model.DeployInstruction, secretKeys []string) (*model.HelmValueDiff, error) {
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

	currentValues := scrubSecrets(di.Values, secretKeys)
	previousValues := scrubSecrets(prev.Values, secretKeys)

	opts := jsondiff.DefaultHTMLOptions()
	opts.Indent = "\t"
	opts.PrintTypes = true
	opts.SkipMatches = true
	diff, diff2 := jsondiff.Compare(previousValues, currentValues, &opts)
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

func scrubSecrets(data []byte, secretKeys []string) []byte {
	if len(secretKeys) == 0 {
		return data
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}
	for _, key := range secretKeys {
		parts, err := featureutil.SmartDotSplit(key)
		if err != nil {
			continue
		}
		scrubPath(obj, parts)
	}
	result, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return result
}

func scrubPath(obj map[string]any, parts []string) {
	for i, part := range parts {
		if i == len(parts)-1 {
			if _, ok := obj[part]; ok {
				obj[part] = "••••••••"
			}
			return
		}
		next, ok := obj[part].(map[string]any)
		if !ok {
			return
		}
		obj = next
	}
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
