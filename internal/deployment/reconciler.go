package deployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ReconcileTriggerEvent struct{}

type Reconciler struct {
	repo             database.Repo
	log              logrus.FieldLogger
	reconcileTrigger <-chan ReconcileTriggerEvent
	deployer         *Deployer

	lock sync.Mutex

	// Metrics
	reconcileTime metric.Int64Histogram
}

func NewReconciler(
	repo database.Repo,
	deployer *Deployer,
	reconcileTrigger <-chan ReconcileTriggerEvent,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*Reconciler, error) {
	reconcileTime, err := meter.Int64Histogram("deployment_reconcile_time", metric.WithDescription("Time spent reconciling"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile time histogram: %w", err)
	}

	return &Reconciler{
		repo:             repo,
		log:              log,
		reconcileTrigger: reconcileTrigger,
		deployer:         deployer,
		reconcileTime:    reconcileTime,
	}, nil
}

func TriggerReconcile(event ReconcileTriggerEvent, trigger chan<- ReconcileTriggerEvent, log logrus.FieldLogger) {
	select {
	case trigger <- event:
	default:
		log.Debug("there is already a reconcile event queued, skipping")
	}
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	for {
		r.log.Debug("reconciling")
		if err := r.Reconcile(ctx); err != nil {
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

func (r *Reconciler) Reconcile(ctx context.Context) error {
	ctx = auth.SetEmail(ctx, "system:deployment_reconciler")

	if shouldrun := r.lock.TryLock(); !shouldrun {
		return nil
	}
	defer r.lock.Unlock()

	tenantEnvironments, err := r.repo.TenantEnvironments(ctx, true)
	if err != nil {
		return fmt.Errorf("get tenant environments: %w", err)
	}

	for _, environment := range tenantEnvironments {
		if err := r.reconcileEnvironment(ctx, environment); err != nil {
			return fmt.Errorf("reconcile environment %q for tenant: %q: %w", environment.Name, environment.TenantName, err)
		}
	}
	return nil
}

func (r *Reconciler) reconcileEnvironment(ctx context.Context, environment *model.TenantEnvironment) error {
	start := time.Now()
	defer func() {
		attrs := attribute.NewSet(
			attribute.String("tenant", environment.TenantName),
			attribute.String("environment", environment.Name),
		)
		r.reconcileTime.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributeSet(attrs))
	}()

	return r.deployer.deploymentsInEnvironment(ctx, environment)
}
