package database

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
)

type DeploymentRepo interface {
	V3DeploymentCreate(ctx context.Context, featureName, featureVersion, description string, ref *model.GHRef, target environment.Labels, ci bool) (*gensql.Deployment, error)
	V3DeploymentGet(ctx context.Context, deploymentID uuid.UUID) (*model.Deployment, error)
	V3DeploymentsGet(ctx context.Context) ([]*model.Deployment, error)
	V3DeploymentStatusesGet(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error)
	V3DeploymentsGetByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error)
	V3DeploymentDelete(ctx context.Context, deploymentID uuid.UUID) error
	V3DeploymentsForEnvironmentToReconcile(ctx context.Context, environmentID uuid.UUID) ([]*model.Deployment, error)
	V3DeploymentStatusCreateOrUpdate(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error
	V3MissingDependencies(ctx context.Context, dependencies []string, environmentID uuid.UUID) ([]string, error)
	V3GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error)
	V3GetEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]*model.FeatureState, error)
	V3InsertEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, deploymentID uuid.UUID, featureName, featureVersion string) error
	GetCIEnvironmentsForTarget(ctx context.Context, labels environment.Labels) ([]*model.TenantEnvironment, error)
	LatestStatusForDeploymentInEnvironment(ctx context.Context, deploymentID, environmentID uuid.UUID) (model.DeploymentStatusState, error)
}

func (r *repo) LatestStatusForDeploymentInEnvironment(ctx context.Context, deploymentID, environmentID uuid.UUID) (model.DeploymentStatusState, error) {
	status, err := r.querier.LatestStatusForDeploymentInEnvironment(ctx, gensql.LatestStatusForDeploymentInEnvironmentParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
	})
	if err != nil {
		return model.DeploymentStatusStateUnknown, err
	}

	return model.DeploymentStatusState(strings.ToUpper(status)), nil
}

func (r *repo) GetCIEnvironmentsForTarget(ctx context.Context, labels environment.Labels) ([]*model.TenantEnvironment, error) {
	envs, err := r.querier.GetCIEnvironmentsForTarget(ctx, labels)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.TenantEnvironment, len(envs))
	for i, e := range envs {
		ret[i] = &model.TenantEnvironment{
			Environment: model.Environment{
				ID:           e.Environment.ID,
				Name:         e.Environment.Name,
				CI:           e.Environment.Ci,
				Description:  nullStringToPtr(e.Environment.Description),
				Created:      e.Environment.Created.Time,
				LastModified: e.Environment.LastModified.Time,
				Kind:         model.EnvironmentKind(e.Environment.Kind),
			},
			TenantName: e.TenantName,
			TenantID:   e.Environment.TenantID,
		}
	}
	return ret, nil
}

func (r *repo) V3DeploymentStatusesGet(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error) {
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

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	return feature, nil
}

func (r *repo) V3GetEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]*model.FeatureState, error) {
	features, err := r.querier.GetEnvironmentFeatures(ctx, environmentID)
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

func (r *repo) V3DeploymentGet(ctx context.Context, deploymentID uuid.UUID) (*model.Deployment, error) {
	row, err := r.querier.DeploymentGet(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return deploymentFromSQL(row.Deployment, row.FeatureDatum)
}

func (r *repo) V3DeploymentsGet(ctx context.Context) ([]*model.Deployment, error) {
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

func (r *repo) V3DeploymentsGetByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error) {
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

func (r *repo) V3DeploymentsForEnvironmentToReconcile(ctx context.Context, environmentID uuid.UUID) ([]*model.Deployment, error) {
	rows, err := r.querier.DeploymentsForEnvironmentToReconcile(ctx, environmentID)
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

func (r *repo) V3DeploymentCreate(ctx context.Context, featureName, featureVersion, description string, ref *model.GHRef, target environment.Labels, ci bool) (*gensql.Deployment, error) {
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
		Description: pgtype.Text{
			String: description,
			Valid:  description != "",
		},
		Ci: ci,
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
