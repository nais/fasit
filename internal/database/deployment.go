package database

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
)

type DeploymentRepo interface {
	V3DeploymentCreate(ctx context.Context, featureName, featureVersion string, ref *model.GHRef, target environment.Labels) (*gensql.Deployment, error)
	V3DeploymentGet(ctx context.Context, deploymentID uuid.UUID) (*Deployment, error)
	V3DeploymentsGet(ctx context.Context) ([]*model.Deployment, error)
	V3DeploymentStatusesGet(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error)
	V3DeploymentsGetByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error)
	V3DeploymentDelete(ctx context.Context, deploymentID uuid.UUID) error
	V3DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]Deployment, error)
	V3DeploymentStatusCreateOrUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error
	V3MissingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error)
	V3GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error)
	V3InsertEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, deploymentID uuid.UUID, featureName, featureVersion string) error
}

func (r *repo) V3DeploymentStatusesGet(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error) {
	rows, err := r.querier.DeploymentStatusGet(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment statuses: %w", err)
	}

	models := make([]*model.DeploymentStatus, len(rows))
	for i, status := range rows {
		models[i] = &model.DeploymentStatus{
			State:         model.DeploymentStatusState(status.Status),
			Message:       status.Message,
			LastModified:  status.LastModified.Time,
			Created:       status.Created.Time,
			DeploymentID:  status.DeploymentID,
			EnvironmentID: status.EnvironmentID,
		}
	}

	return models, nil
}

func (r *repo) V3DeploymentDelete(ctx context.Context, deploymentID uuid.UUID) error {
	return r.querier.DeploymentDelete(ctx, deploymentID)
}

func (r *repo) V3InsertEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, deploymentID uuid.UUID, featureName, featureVersion string) error {
	return r.querier.InsertEnvironmentFeature(ctx, gensql.InsertEnvironmentFeatureParams{
		EnvironmentID:  environmentID,
		DeploymentID:   deploymentID,
		FeatureName:    featureName,
		FeatureVersion: featureVersion,
	})
}

func (r *repo) V3GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error) {
	f, err := r.querier.GetEnvironmentFeature(ctx, gensql.GetEnvironmentFeatureParams{
		EnvironmentID: environmentID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	fyaml, defaultValues, err := makeFeatureYAML(f.Kinds, f.Dependencies, f.Values, f.DefaultValues, nil, f.Timeout)
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
		// if exists in the environment_features table, it must have deployments
		HasDeployments: true,
	}, nil
}

func (r *repo) V3MissingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error) {
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

	ID       uuid.UUID                 `json:"id"`
	Created  time.Time                 `json:"created"`
	Target   environment.Labels        `json:"target"`
	Statuses []gensql.DeploymentStatus `json:"statuses"`
}

func (r *repo) V3DeploymentGet(ctx context.Context, deploymentID uuid.UUID) (*Deployment, error) {
	row, err := r.querier.DeploymentGet(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	feature, err := featureFromSQL(gensql.FeatureDeploymentsForEnvironmentRow{
		Deployment:    row.Deployment,
		Name:          row.Name,
		Version:       row.Version,
		Chart:         row.Chart,
		Description:   row.Description,
		Source:        row.Source,
		Kinds:         row.Kinds,
		Dependencies:  row.Dependencies,
		Values:        row.Values,
		DefaultValues: row.DefaultValues,
		Timeout:       row.Timeout,
	})
	if err != nil {
		return nil, err
	}
	statuses, err := r.querier.DeploymentStatusGet(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return &Deployment{
		Feature:  feature,
		ID:       row.Deployment.ID,
		Created:  row.Deployment.Created.Time,
		Target:   row.Deployment.Target,
		Statuses: statuses,
	}, nil
}

func (r *repo) V3DeploymentsGet(ctx context.Context) ([]*model.Deployment, error) {
	rows, err := r.querier.DeploymentsGet(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Deployment, len(rows))
	for i, r := range rows {
		fyaml, defaultValues, err := makeFeatureYAML(r.Kinds, r.Dependencies, r.Values, r.DefaultValues, nil, r.Timeout)
		if err != nil {
			return nil, fmt.Errorf("make feature yaml: %w", err)
		}

		target := make([]*model.EnvironmentLabel, 0)
		for k, v := range r.Deployment.Target {
			target = append(target, &model.EnvironmentLabel{
				Key:   k,
				Value: v,
			})
		}

		feature := &model.Feature{
			Name:        r.Name,
			Description: r.Description,
			Version:     r.Version,
			Chart:       r.Chart,
			Source:      r.Source,
			FeatureYAML: fyaml,
			ValuesYAML:  defaultValues,
			SpecVersion: "v2",
			// if exists in the environment_features table, it must have deployments
			HasDeployments: true,
		}
		ret[i] = &model.Deployment{
			Feature: feature,
			ID:      r.Deployment.ID,
			Created: r.Deployment.Created.Time,
			Target:  target,
		}
	}

	return ret, nil
}

func (r *repo) V3DeploymentsGetByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error) {
	rows, err := r.querier.DeploymentsGetByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Deployment, len(rows))
	for i, r := range rows {
		fyaml, defaultValues, err := makeFeatureYAML(r.Kinds, r.Dependencies, r.Values, r.DefaultValues, nil, r.Timeout)
		if err != nil {
			return nil, fmt.Errorf("make feature yaml: %w", err)
		}

		target := make([]*model.EnvironmentLabel, 0)
		for k, v := range r.Deployment.Target {
			target = append(target, &model.EnvironmentLabel{
				Key:   k,
				Value: v,
			})
		}

		feature := &model.Feature{
			Name:        r.Name,
			Description: r.Description,
			Version:     r.Version,
			Chart:       r.Chart,
			Source:      r.Source,
			FeatureYAML: fyaml,
			ValuesYAML:  defaultValues,
			SpecVersion: "v2",
			// if exists in the environment_features table, it must have deployments
			HasDeployments: true,
		}
		ret[i] = &model.Deployment{
			Feature: feature,
			ID:      r.Deployment.ID,
			Created: r.Deployment.Created.Time,
			Target:  target,
		}
	}

	return ret, nil
}

func (r *repo) V3DeploymentsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]Deployment, error) {
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
			Statuses: []gensql.DeploymentStatus{
				{
					DeploymentID:  row.Deployment.ID,
					EnvironmentID: environmentID,
					Status:        row.Status.String,
					Message:       row.StatusMessage.String,
					LastModified:  row.StatusLastModified,
					Created:       row.StatusCreated,
				},
			},
		}
	}

	return ret, nil
}

func featureFromSQL(f gensql.FeatureDeploymentsForEnvironmentRow) (*model.Feature, error) {
	kinds := make([]string, len(f.Kinds))
	copy(kinds, f.Kinds)

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

func (r *repo) V3DeploymentCreate(ctx context.Context, featureName, featureVersion string, ref *model.GHRef, target environment.Labels) (*gensql.Deployment, error) {
	var ghRef []byte
	if ref != nil {
		b, err := json.Marshal(ref)
		if err != nil {
			return nil, fmt.Errorf("marshal gh ref: %w", err)
		}

		ghRef = b
	}
	ret, err := r.querier.DeploymentCreate(ctx, gensql.DeploymentCreateParams{
		FeatureName: featureName,
		Version:     featureVersion,
		GhRef:       ghRef,
		Target:      target,
	})

	return &ret, err
}

func (r *repo) FeatureEnabled(ctx context.Context, featureName string, envID uuid.UUID) (bool, error) {
	return r.querier.FeatureEnabled(ctx, gensql.FeatureEnabledParams{
		FeatureName:   featureName,
		EnvironmentID: envID,
	})
}

func (r *repo) V3DeploymentStatusCreateOrUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error {
	return r.querier.DeploymentStatusCreateOrUpdate(ctx, gensql.DeploymentStatusCreateOrUpdateParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
		Status:        status.String(),
		Message:       message,
	})
}
