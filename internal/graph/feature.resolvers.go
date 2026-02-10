package graph

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// Environment is the resolver for the environment field.
func (r *configOverrideResolver) Environment(ctx context.Context, obj *model.ConfigOverride) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

// ActiveVersion is the resolver for the activeVersion field.
func (r *featureResolver) ActiveVersion(ctx context.Context, obj *model.Feature) (string, error) {
	di, err := r.Repo.DeployInstructionsLatestForFeature(ctx, obj.GraphVars.EnvironmentID, obj.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get latest deploy instruction: %w", err)
	}

	return di.FeatureVersion, nil
}

// Dependencies is the resolver for the dependencies field.
func (r *featureResolver) Dependencies(ctx context.Context, obj *model.Feature) ([]*model.Dependency, error) {
	return obj.Dependencies, nil
}

// Configoverrides is the resolver for the configoverrides field.
func (r *featureResolver) Configoverrides(ctx context.Context, obj *model.Feature) ([]*model.ConfigOverride, error) {
	return r.Repo.ConfigOverridesByFeature(ctx, obj.Name)
}

// Configuration is the resolver for the configuration field.
func (r *featureResolver) Configuration(ctx context.Context, obj *model.Feature) (*model.Configurations, error) {
	return &model.Configurations{
		FeatureName: obj.Name,
		EnvID:       &obj.GraphVars.EnvironmentID,
		RolloutID:   obj.GraphVars.RolloutID,
	}, nil
}

// State is the resolver for the state field.
func (r *featureResolver) State(ctx context.Context, obj *model.Feature) (*model.FeatureState, error) {
	if obj.GraphVars.EnvironmentID == uuid.Nil {
		return nil, nil
	}

	env, err := r.Repo.EnvironmentGet(ctx, obj.GraphVars.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}
	ok := false
	for _, kind := range obj.EnvironmentKinds {
		if env.Kind == kind {
			ok = true
			break
		}
	}

	if !ok {
		// Not a valid environment for this feature
		return nil, nil
	}

	return featurepkg.FeatureStateGet(ctx, obj.GraphVars.EnvironmentID, obj.Name)
}

// Status is the resolver for the status field.
func (r *featureResolver) Status(ctx context.Context, obj *model.Feature) (*model.Status, error) {
	di, err := r.Repo.DeployInstructionsLatestForFeature(ctx, obj.GraphVars.EnvironmentID, obj.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			f, err := featurepkg.FeatureByNameForEnv(ctx, obj.Name, obj.GraphVars.EnvironmentID)
			if err != nil {
				return nil, fmt.Errorf("feature %v not found: %v", obj.Name, err)
			}
			return &model.Status{
				EnvironmentID: obj.GraphVars.EnvironmentID,
				Feature:       obj.Name,
				Version:       f.Version,
				Status:        model.RolloutStatusUnknown,
				Created:       time.Now(),
				LastModified:  time.Now(),
				Log:           "",
			}, nil
		}
		return nil, err
	}

	return &model.Status{
		EnvironmentID:       di.EnvironmentID,
		Feature:             di.FeatureName,
		Version:             di.FeatureVersion,
		Status:              di.Status,
		ConfigHash:          di.Hash,
		Created:             di.Created,
		LastModified:        di.LastModified,
		DeployInstructionID: di.ID,
	}, nil
}

// Histories is the resolver for the histories field.
func (r *featureResolver) Histories(ctx context.Context, obj *model.Feature) ([]*model.FeatureHistory, error) {
	dis, err := r.Repo.DeployInstructionsForFeature(ctx, obj.GraphVars.EnvironmentID, obj.Name, 1)
	if err != nil {
		return nil, err
	}

	history := make([]*model.FeatureHistory, len(dis))
	for i, di := range dis {
		history[i] = &model.FeatureHistory{
			ID:           di.ID,
			Version:      di.FeatureVersion,
			Status:       di.Status,
			Created:      di.Created,
			LastModified: di.LastModified,
			Di:           di,
		}
	}

	return history, nil
}

// HelmValueDiff is the resolver for the helmValueDiff field.
func (r *featureResolver) HelmValueDiff(ctx context.Context, obj *model.Feature) (*model.HelmValueDiff, error) {
	di, err := r.Repo.DeployInstructionsLatestForFeature(ctx, obj.GraphVars.EnvironmentID, obj.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.HelmValueDiff{
				Difference: model.HelmValueDifferenceNoMatch,
			}, nil
		}
		return nil, fmt.Errorf("get latest deploy instruction: %w", err)
	}

	prev, err := r.Repo.HelmValueDiffGet(ctx, di)
	if err != nil {
		return nil, fmt.Errorf("get value diff: %w", err)
	}

	return prev, nil
}

// Log is the resolver for the log field.
func (r *featureHistoryResolver) Log(ctx context.Context, obj *model.FeatureHistory) ([]*model.LogLine, error) {
	return r.Repo.LogsGet(ctx, obj.ID)
}

// HelmValueDiff is the resolver for the helmValueDiff field.
func (r *featureHistoryResolver) HelmValueDiff(ctx context.Context, obj *model.FeatureHistory) (*model.HelmValueDiff, error) {
	return r.Repo.HelmValueDiffGet(ctx, obj.Di)
}

// Features is the resolver for the features field.
func (r *queryResolver) Features(ctx context.Context) ([]*model.Feature, error) {
	return featurepkg.Features(ctx)
}

// Feature is the resolver for the feature field.
func (r *queryResolver) Feature(ctx context.Context, name string) (*model.Feature, error) {
	f, err := featurepkg.FeatureByName(ctx, name)

	if errors.Is(err, pgx.ErrNoRows) {
		return featurepkg.RolloutByName(ctx, name)
	}

	return f, err
}

// History is the resolver for the history field.
func (r *queryResolver) History(ctx context.Context, id uuid.UUID) (*model.FeatureHistory, error) {
	di, err := r.Repo.DeployInstructionGet(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.FeatureHistory{
		ID:           di.ID,
		Version:      di.FeatureVersion,
		Status:       di.Status,
		Created:      di.Created,
		LastModified: di.LastModified,
		Di:           di,
	}, nil
}

func (r *Resolver) ConfigOverride() graphgen.ConfigOverrideResolver {
	return &configOverrideResolver{r}
}

func (r *Resolver) Feature() graphgen.FeatureResolver { return &featureResolver{r} }

func (r *Resolver) FeatureHistory() graphgen.FeatureHistoryResolver {
	return &featureHistoryResolver{r}
}

type (
	configOverrideResolver struct{ *Resolver }
	featureResolver        struct{ *Resolver }
	featureHistoryResolver struct{ *Resolver }
)
