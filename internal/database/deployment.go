package database

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
)

type DeploymentRepo interface {
	DeploymentCreate(ctx context.Context, featureName, featureVersion string, ghRef []byte, target environment.Labels, hash string) (*gensql.Deployment, error)
	DeploymentTargetsGetAll(ctx context.Context) ([]gensql.DeploymentTargetsGetAllRow, error)
	DeploymentTargetsGet(ctx context.Context, deploymentID uuid.UUID) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsGetPending(ctx context.Context) ([]gensql.DeploymentTarget, error)
	DeploymentTargetsCreate(ctx context.Context, deploymentID, environmentID uuid.UUID) error
	DeploymentTargetsUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status string) error
	DeploymentsGet(ctx context.Context) ([]gensql.Deployment, error)
	DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]Deployment, error)
	FeatureEnabled(ctx context.Context, featureName string, envID uuid.UUID) (bool, error)
	MissingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error)
}

func (r *repo) MissingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error) {
	if len(dependencies) == 0 {
		return []string{}, nil
	}
	deployedFeatures, err := r.querier.DeployInstructionsGetDeployedFeatures(ctx, gensql.DeployInstructionsGetDeployedFeaturesParams{
		FeatureNames:  dependencies,
		EnvironmentID: environmentID,
	})
	if err != nil {
		return nil, err
	}

	missing := make([]string, 0)
	for _, d := range dependencies {
		if !slices.Contains(deployedFeatures, d) {
			missing = append(missing, d)
		}
	}
	return missing, nil
}

type Deployment struct {
	*model.Feature

	ID      uuid.UUID
	Created time.Time
	Target  environment.Labels
	Hash    string
}

func (r *repo) DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]Deployment, error) {
	rows, err := r.querier.FeatureDeploymentsForEnvironment(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	ret := make([]Deployment, len(rows))
	for i, row := range rows {
		feature, err := featureFromSQL(row)
		if err != nil {
			return nil, err
		}

		ret[i] = Deployment{
			Feature: feature,
			ID:      row.Deployment.ID,
			Created: row.Deployment.Created.Time,
			Target:  row.Deployment.Target,
			Hash:    row.Deployment.Hash,
		}
	}

	return ret, nil
}

func featureFromSQL(f gensql.FeatureDeploymentsForEnvironmentRow) (*model.Feature, error) {
	kinds := make([]string, len(f.Kinds))
	for i, k := range f.Kinds {
		kinds[i] = string(k)
	}

	fyaml, defaultValues, err := makeFeatureYAML(kinds, f.Dependencies, f.Values, f.DefaultValues, nil, f.Timeout)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
		Name:        f.Name,
		Description: f.Description,
		Version:     f.Version,
		Chart:       f.Chart,
		Source:      f.Source,
		FeatureYAML: fyaml,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
	}, nil
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
