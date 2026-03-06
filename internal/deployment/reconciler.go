package deployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ReconcileTriggerEvent struct{}

type TriggerResult int

const (
	TriggerResultSkipped TriggerResult = iota
	TriggerResultSuccess
	TriggerResultFailed
)

type reconciler struct {
	querier          deploymentsql.Querier
	log              logrus.FieldLogger
	reconcileTrigger chan chan TriggerResult
	deployer         *deployer

	lock sync.Mutex

	// Metrics
	reconcileTime metric.Int64Histogram
}

func newReconciler(querier deploymentsql.Querier, deployer *deployer, meter metric.Meter, log logrus.FieldLogger) (*reconciler, error) {
	reconcileTime, err := meter.Int64Histogram("deployment_reconcile_time", metric.WithDescription("Time spent reconciling"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile time histogram: %w", err)
	}
	reconcileTrigger := make(chan chan TriggerResult, 1)
	return &reconciler{
		querier:          querier,
		log:              log,
		reconcileTrigger: reconcileTrigger,
		deployer:         deployer,
		reconcileTime:    reconcileTime,
	}, nil
}

func (r *reconciler) trigger(_ ReconcileTriggerEvent) chan TriggerResult {
	done := make(chan TriggerResult, 1)
	select {
	case r.reconcileTrigger <- done:
	default:
		r.log.Debug("there is already a reconcile event queued, skipping")
		done <- TriggerResultSkipped
	}
	return done
}

func (r *reconciler) Run(ctx context.Context, interval time.Duration) {
	var done chan TriggerResult
	for {
		r.log.Info("reconciling")
		err := r.Reconcile(ctx)
		if err != nil {
			r.log.WithError(err).Error("reconcile")
		}
		if done != nil {
			if err != nil {
				done <- TriggerResultFailed
			} else {
				done <- TriggerResultSuccess
			}
			done = nil
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		case done = <-r.reconcileTrigger:
			r.log.Info("manual reconcile triggered")
		}
	}
}

func (r *reconciler) Reconcile(ctx context.Context) error {
	ctx = auth.SetEmail(ctx, "system:deployment_reconciler")

	if shouldrun := r.lock.TryLock(); !shouldrun {
		return nil
	}
	defer r.lock.Unlock()

	tenantEnvironments, err := environment.TenantEnvironments(ctx, true)
	if err != nil {
		return fmt.Errorf("get tenant environments: %w", err)
	}

	r.log.WithField("num_envs", len(tenantEnvironments)).Info("reconciling tenant environments")
	for _, environment := range tenantEnvironments {
		if err := r.reconcileEnvironment(ctx, environment); err != nil {
			return fmt.Errorf("reconcile environment %q for tenant: %q: %w", environment.Name, environment.TenantName, err)
		}
	}
	return nil
}

func (r *reconciler) reconcileEnvironment(ctx context.Context, environment *model.TenantEnvironment) error {
	start := time.Now()
	defer func() {
		attrs := attribute.NewSet(
			attribute.String("tenant", environment.TenantName),
			attribute.String("environment", environment.Name),
		)
		r.reconcileTime.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributeSet(attrs))
	}()

	publisher := r.deployer.publisher(naisdTopicID(environment.TenantName, environment.Name), r.deployer.log)
	defer publisher.Stop()

	allDeployments, err := r.listDeploymentsToReconcile(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	filtered := filterDeployments(allDeployments)

	r.log.
		WithField("tenant", environment.TenantName).
		WithField("env", environment.Name).
		WithField("num_deployments", len(filtered)).
		Info("deployments to reconcile for environment")
	for _, deployment := range filtered {
		if err := r.deployer.deployToEnvironment(ctx, deployment, environment, publisher); err != nil {
			// TODO: continue on error? earlier we returned the error immediately when the above was inside the loop.
			return err
		}
	}
	return nil
}

func (r *reconciler) listDeploymentsToReconcile(ctx context.Context, environmentID uuid.UUID) ([]*Deployment, error) {
	rows, err := r.querier.ListDeploymentsToReconcile(ctx, environmentID)
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
