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

type reconciler struct {
	querier          deploymentsql.Querier
	log              logrus.FieldLogger
	reconcileTrigger chan struct{}
	deployer         *deployer

	lock sync.Mutex

	// Metrics
	reconcileTime     metric.Int64Histogram
	reconcileLoopTime metric.Int64Histogram
}

func newReconciler(querier deploymentsql.Querier, deployer *deployer, meter metric.Meter, log logrus.FieldLogger) (*reconciler, error) {
	reconcileTime, err := meter.Int64Histogram("deployment_reconcile_time", metric.WithDescription("Time spent reconciling per environment"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile time histogram: %w", err)
	}
	reconcileLoopTime, err := meter.Int64Histogram("reconcile_loop_duration", metric.WithDescription("Total time for one full reconcile loop"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile loop time histogram: %w", err)
	}
	reconcileTrigger := make(chan struct{}, 1)
	return &reconciler{
		querier:           querier,
		log:               log,
		reconcileTrigger:  reconcileTrigger,
		deployer:          deployer,
		reconcileTime:     reconcileTime,
		reconcileLoopTime: reconcileLoopTime,
	}, nil
}

func (r *reconciler) Run(ctx context.Context, interval time.Duration) {
	for {
		r.log.Info("reconciling")
		err := r.Reconcile(ctx)
		if err != nil {
			r.log.WithError(err).Error("reconcile")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		case <-r.reconcileTrigger:
			r.log.Info("manual reconcile triggered")
		}
	}
}

func (r *reconciler) Reconcile(ctx context.Context) error {
	loopStart := time.Now()
	ctx = auth.SetEmail(ctx, "system:deployment_reconciler")

	if shouldrun := r.lock.TryLock(); !shouldrun {
		return nil
	}
	defer r.lock.Unlock()

	tenantEnvironments, err := environment.ListTenantEnvironments(ctx, true)
	if err != nil {
		return fmt.Errorf("get tenant environments: %w", err)
	}

	r.log.WithField("num_envs", len(tenantEnvironments)).Info("reconciling tenant environments")
	for _, environment := range tenantEnvironments {
		if err := r.reconcileEnvironment(ctx, environment); err != nil {
			return fmt.Errorf("reconcile environment %q for tenant: %q: %w", environment.Name, environment.TenantName, err)
		}
	}

	r.reconcileLoopTime.Record(ctx, time.Since(loopStart).Milliseconds())
	return nil
}

func (r *reconciler) listDeploymentsToReconcile(ctx context.Context, environmentID uuid.UUID) ([]*Deployment, error) {
	rows, err := r.querier.ListDeploymentsForEnvironment(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	deps, err := deploymentsFromRows(rows)
	if err != nil {
		return nil, err
	}

	enabled := make([]*Deployment, 0, len(deps))
	for _, dep := range deps {
		if !dep.FeatureDisabled {
			enabled = append(enabled, dep)
		}
	}

	return mostSpecificPerFeature(enabled), nil
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

	publisher := r.deployer.publisher(naisdTopicID(environment.TenantName, environment.Name))

	deployments, err := r.listDeploymentsToReconcile(ctx, environment.ID)
	if err != nil {
		return fmt.Errorf("get deployments for environment %q: %w", environment.Name, err)
	}

	r.log.
		WithField("tenant", environment.TenantName).
		WithField("env", environment.Name).
		WithField("num_deployments", len(deployments)).
		Info("deployments to reconcile for environment")
	for _, deployment := range deployments {
		if err := r.deployer.deployToEnvironment(ctx, deployment, environment, publisher); err != nil {
			return err
		}
	}
	return nil
}
