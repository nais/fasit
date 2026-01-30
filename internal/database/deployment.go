package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type DeploymentRepo interface {
	GetDeployment(ctx context.Context, deploymentID uuid.UUID) (*model.Deployment, error)
	ListDeployments(ctx context.Context) ([]*model.Deployment, error)
	ListDeploymentStatuses(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error)
	ListDeploymentsByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error)
	DeleteDeployment(ctx context.Context, deploymentID uuid.UUID) error
	SetDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error
	GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error)
	ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]*model.FeatureState, error)
}

func (r *repo) ListDeploymentStatuses(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error) {
	rows, err := r.querier.DeploymentStatusGet(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment statuses: %w", err)
	}

	models := make([]*model.DeploymentStatus, len(rows))
	for i, status := range rows {
		models[i] = &model.DeploymentStatus{
			State:         model.DeploymentStatusState(strings.ToUpper(status.Status)),
			Message:       status.Message,
			LastModified:  status.LastModified.Time,
			Created:       status.Created.Time,
			DeploymentID:  status.DeploymentID,
			EnvironmentID: status.EnvironmentID,
		}
	}

	return models, nil
}

func (r *repo) DeleteDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	return r.querier.DeploymentDelete(ctx, deploymentID)
}

func (r *repo) GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error) {
	f, err := r.querier.GetEnvironmentFeature(ctx, gensql.GetEnvironmentFeatureParams{
		EnvironmentID: environmentID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	return feature, nil
}

func (r *repo) ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]*model.FeatureState, error) {
	features, err := r.querier.ListEnvironmentFeatures(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.FeatureState, len(features))
	for i, f := range features {
		ret[i] = &model.FeatureState{
			ID:           environmentID.String() + "-" + f.FeatureDatum.Name,
			FeatureName:  f.FeatureDatum.Name,
			Enabled:      true,
			EnabledAt:    &f.Created.Time,
			Created:      f.Created.Time,
			LastModified: f.Created.Time,
			EnvID:        environmentID,
		}
	}

	return ret, nil
}

func (r *repo) GetDeployment(ctx context.Context, deploymentID uuid.UUID) (*model.Deployment, error) {
	row, err := r.querier.DeploymentGet(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return deploymentFromSQL(row.Deployment, row.FeatureDatum)
}

func (r *repo) ListDeployments(ctx context.Context) ([]*model.Deployment, error) {
	rows, err := r.querier.DeploymentsGet(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Deployment, len(rows))
	for i, row := range rows {
		deployment, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		ret[i] = deployment
	}

	return ret, nil
}

func (r *repo) ListDeploymentsByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error) {
	rows, err := r.querier.DeploymentsGetByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Deployment, len(rows))
	for i, row := range rows {
		deployment, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		ret[i] = deployment
	}

	return ret, nil
}

func (r *repo) FeatureEnabled(ctx context.Context, featureName string, envID uuid.UUID) (bool, error) {
	return r.querier.FeatureEnabled(ctx, gensql.FeatureEnabledParams{
		FeatureName:   featureName,
		EnvironmentID: envID,
	})
}

// TODO: receiver is still using this
func (r *repo) SetDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error {
	return r.querier.DeploymentStatusCreateOrUpdate(ctx, gensql.DeploymentStatusCreateOrUpdateParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
		Status:        status.String(),
		Message:       message,
	})
}

func featureFromSQL(f gensql.FeatureDatum) (*model.Feature, error) {
	fyaml, defaultValues, err := makeFeatureYAML(f)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
		FeatureYAML: fyaml,
		Name:        f.Name,
		Chart:       f.Chart,
		Version:     f.Version,
		Description: f.Description,
		Source:      f.Source,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
	}, nil
}

func deploymentFromSQL(d gensql.Deployment, fd gensql.FeatureDatum) (*model.Deployment, error) {
	feature, err := featureFromSQL(fd)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	var desc *string
	if d.Description.Valid {
		desc = &d.Description.String
	}

	return &model.Deployment{
		ID:           d.ID,
		Feature:      feature,
		Description:  desc,
		Created:      d.Created.Time,
		CI:           d.Ci,
		TargetLabels: d.Target,
	}, nil
}
