package deployment

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Manager struct {
	deployer   *deployer
	reconciler *reconciler
}

// TODO: check if we can use same request as in graphql
type Request struct {
	Chart       string             `json:"chart"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Ref         *model.GHRef       `json:"ref"`
	Global      bool               `json:"global"`
	Target      environment.Labels `json:"target"`
	CI          string             `json:"ci,omitempty"`
}

func NewManager(repo database.Repo, publisher NewPublisher, m metric.Meter, log logrus.FieldLogger) (*Manager, error) {
	d, err := newDeployer(repo, publisher, m, log.WithField("subsystem", "deployer"))
	if err != nil {
		return nil, err
	}

	r, err := newReconciler(repo, d, m, log.WithField("subsystem", "reconciler"))
	if err != nil {
		return nil, err
	}
	return &Manager{
		deployer:   d,
		reconciler: r,
	}, nil
}

func (dm *Manager) Run(ctx context.Context, intervalSeconds time.Duration) {
	dm.reconciler.Run(ctx, intervalSeconds)
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

func (dm *Manager) GetDeployment(ctx context.Context, id uuid.UUID) (*database.Deployment, error) {
	return dm.deployer.GetDeployment(ctx, id)
}

func (dm *Manager) CreateDeployment(ctx context.Context, req *Request) (uuid.UUID, error) {
	return dm.deployer.CreateDeployment(ctx, req)
}
