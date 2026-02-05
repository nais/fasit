package deployment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

const managerKey ctxKey = iota

func Register(ctx context.Context, deploymentManager *Manager) context.Context {
	return context.WithValue(ctx, managerKey, deploymentManager)
}

func fromContext(ctx context.Context) *Manager {
	return ctx.Value(managerKey).(*Manager)
}

// TriggerReconcile will trigger an asynchronous reconciliation of deployments. The returned channel can be used to wait
// for the result.
func TriggerReconcile(ctx context.Context, event ReconcileTriggerEvent) chan TriggerResult {
	return fromContext(ctx).reconciler.trigger(event)
}

func CreateDeployment(ctx context.Context, req Request) (uuid.UUID, error) {
	feat, err := fromContext(ctx).chartDownloader(req.Chart, req.Version)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to convert oci chart: %w", err)
	}

	if len(feat.EnvironmentKinds) == 0 {
		return uuid.Nil, fmt.Errorf("no environments defined in Feature.yaml")
	}

	if feat.Source == "" {
		return uuid.Nil, fmt.Errorf("no source url found in Chart.yaml")
	}

	if !req.SkipCI {
		if err := fromContext(ctx).deployer.deployToCI(ctx, feat, req); err != nil {
			return uuid.Nil, fmt.Errorf("deploy to ci: %w", err)
		}
	}

	return fromContext(ctx).deployer.CreateDeployment(ctx, feat, req, false)
}

func GetDeployment(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	return getDeployment(ctx, fromContext(ctx).querier, id)
}

func ListDeployments(ctx context.Context) ([]*Deployment, error) {
	rows, err := fromContext(ctx).querier.ListDeployments(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*Deployment, len(rows))
	for i, row := range rows {
		deployment, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		ret[i] = deployment
	}

	return ret, nil
}

func ListDeploymentStatuses(ctx context.Context, deploymentID uuid.UUID) ([]*DeploymentStatus, error) {
	rows, err := fromContext(ctx).querier.ListDeploymentStatuses(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment statuses: %w", err)
	}

	models := make([]*DeploymentStatus, len(rows))
	for i, status := range rows {
		models[i] = &DeploymentStatus{
			State:         DeploymentStatusState(strings.ToUpper(status.Status)),
			Message:       status.Message,
			LastModified:  status.LastModified.Time,
			Created:       status.Created.Time,
			DeploymentID:  status.DeploymentID,
			EnvironmentID: status.EnvironmentID,
		}
	}

	return models, nil
}

func ListDeploymentsByFeature(ctx context.Context, featureName string) ([]*Deployment, error) {
	rows, err := fromContext(ctx).querier.ListDeploymentsByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	ret := make([]*Deployment, len(rows))
	for i, row := range rows {
		deployment, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		ret[i] = deployment
	}

	return ret, nil
}

func DeleteDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	return fromContext(ctx).querier.DeleteDeployment(ctx, deploymentID)
}

func SetDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error {
	return fromContext(ctx).querier.SetDeploymentStatus(ctx, deploymentsql.SetDeploymentStatusParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
		Status:        status.String(),
		Message:       message,
	})
}

func GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error) {
	f, err := fromContext(ctx).querier.GetEnvironmentFeature(ctx, deploymentsql.GetEnvironmentFeatureParams{
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

func ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]*model.FeatureState, error) {
	features, err := fromContext(ctx).querier.ListEnvironmentFeatures(ctx, environmentID)
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
