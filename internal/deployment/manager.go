package deployment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Manager struct {
	deployer        *deployer
	reconciler      *reconciler
	querier         deploymentsql.Querier
	chartDownloader ChartDownloader
}

type ChartDownloader func(chartURL, version string) (*model.Feature, error)

// TODO: check if we can use same request as in graphql
type Request struct {
	Chart       string             `json:"chart"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Ref         *model.GHRef       `json:"ref"`
	Global      bool               `json:"global"`
	Target      environment.Labels `json:"target"`
	SkipCI      bool               `json:"skipCI"`
}

type Option func(*Manager)

func WithChartDownloader(downloader ChartDownloader) Option {
	return func(m *Manager) {
		m.chartDownloader = downloader
	}
}

func NewManager(repo database.Repo, publisher NewPublisher, m metric.Meter, log logrus.FieldLogger, opts ...Option) (*Manager, error) {
	querier := deploymentsql.New(repo.GetConnPool())
	d, err := newDeployer(repo, querier, publisher, m, log.WithField("subsystem", "deployer"))
	if err != nil {
		return nil, err
	}

	r, err := newReconciler(repo, querier, d, m, log.WithField("subsystem", "reconciler"))
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		deployer:   d,
		reconciler: r,
		querier:    querier,
	}
	for _, opt := range opts {
		opt(mgr)
	}

	if mgr.chartDownloader == nil {
		mgr.chartDownloader = model.FromChart
	}

	return mgr, nil
}

func (dm *Manager) Run(ctx context.Context, interval time.Duration) {
	dm.reconciler.Run(ctx, interval)
}

// Reconcile performs a reconciliation of deployments, and will block until complete.
func (dm *Manager) Reconcile(ctx context.Context) error {
	return dm.reconciler.Reconcile(ctx)
}

// TriggerReconcile will trigger an asynchronous reconciliation of deployments. The returned channel can be used to wait
// for the result.
func (dm *Manager) TriggerReconcile(event ReconcileTriggerEvent) chan TriggerResult {
	return dm.reconciler.trigger(event)
}

func (dm *Manager) CreateDeployment(ctx context.Context, req Request) (uuid.UUID, error) {
	feat, err := dm.chartDownloader(req.Chart, req.Version)
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
		if err := dm.deployer.deployToCI(ctx, feat, req); err != nil {
			return uuid.Nil, fmt.Errorf("deploy to ci: %w", err)
		}
	}

	return dm.deployer.CreateDeployment(ctx, feat, req, false)
}

func (dm *Manager) GetDeployment(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	return getDeployment(ctx, dm.querier, id)
}

func (dm *Manager) ListDeployments(ctx context.Context) ([]*model.Deployment, error) {
	rows, err := dm.querier.ListDeployments(ctx)
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

func (dm *Manager) ListDeploymentStatuses(ctx context.Context, deploymentID uuid.UUID) ([]*model.DeploymentStatus, error) {
	rows, err := dm.querier.ListDeploymentStatuses(ctx, deploymentID)
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

func (dm *Manager) ListDeploymentsByFeature(ctx context.Context, featureName string) ([]*model.Deployment, error) {
	rows, err := dm.querier.ListDeploymentsByFeature(ctx, featureName)
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

func (dm *Manager) DeleteDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	return dm.querier.DeleteDeployment(ctx, deploymentID)
}

func (dm *Manager) SetDeploymentStatus(ctx context.Context, deploymentID, environmentID uuid.UUID, status model.RolloutStatus, message string) error {
	return dm.querier.SetDeploymentStatus(ctx, deploymentsql.SetDeploymentStatusParams{
		DeploymentID:  deploymentID,
		EnvironmentID: environmentID,
		Status:        status.String(),
		Message:       message,
	})
	return nil
}

func (dm *Manager) GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error) {
	f, err := dm.querier.GetEnvironmentFeature(ctx, deploymentsql.GetEnvironmentFeatureParams{
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

func (dm *Manager) ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]*model.FeatureState, error) {
	features, err := dm.querier.ListEnvironmentFeatures(ctx, environmentID)
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
