package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Reconciler struct {
	querier reconcilersql.Querier
	log     logrus.FieldLogger
	trigger chan struct{}

	reconcileTime     metric.Int64Histogram
	reconcileLoopTime metric.Int64Histogram

	// Phase durations from the last Reconcile call.
	LastFetchDur  time.Duration
	LastRenderDur time.Duration
}

func New(querier reconcilersql.Querier, meter metric.Meter, log logrus.FieldLogger) (*Reconciler, error) {
	reconcileTime, err := meter.Int64Histogram("reconciler_environment_time", metric.WithDescription("Time spent reconciling per environment"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile time histogram: %w", err)
	}
	reconcileLoopTime, err := meter.Int64Histogram("reconciler_loop_duration", metric.WithDescription("Total time for one full reconcile loop"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile loop time histogram: %w", err)
	}

	return &Reconciler{
		querier:           querier,
		log:               log,
		trigger:           make(chan struct{}, 1),
		reconcileTime:     reconcileTime,
		reconcileLoopTime: reconcileLoopTime,
	}, nil
}

// Run starts the reconcile loop, writing results via the given writer.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration, writer ResultWriter) {
	for {
		r.log.Info("reconciling")
		results, err := r.Reconcile(ctx)
		if err != nil {
			r.log.WithError(err).Error("reconcile")
		} else {
			ioStart := time.Now()
			if err := writer.WriteResults(ctx, results); err != nil {
				r.log.WithError(err).Error("write results")
			}
			r.log.WithField("io", time.Since(ioStart)).Info("results written")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		case <-r.trigger:
			r.log.Info("manual reconcile triggered")
		}
	}
}

func (r *Reconciler) TriggerReconcile() {
	select {
	case r.trigger <- struct{}{}:
	default:
		r.log.Debug("there is already a reconcile event queued, skipping")
	}
}

// Reconcile fetches the current state and renders deploy decisions for all
// environments. It performs no writes — callers decide what to do with the
// returned results (write to DB, display in UI, etc.).
func (r *Reconciler) Reconcile(ctx context.Context) ([]Result, error) {
	loopStart := time.Now()

	fetchStart := time.Now()
	snap, err := r.fetchSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch snapshot: %w", err)
	}
	r.LastFetchDur = time.Since(fetchStart)

	r.log.WithField("num_envs", len(snap.environments)).
		WithField("num_deployments", len(snap.deployments)).
		Info("reconciling tenant environments")

	renderStart := time.Now()
	results := r.renderAll(snap)
	r.LastRenderDur = time.Since(renderStart)

	deployCount := 0
	for _, res := range results {
		if res.Action == ActionDeploy {
			deployCount++
		}
	}
	r.log.WithField("total_results", len(results)).
		WithField("deploy_count", deployCount).
		Info("render complete")

	totalDur := time.Since(loopStart)
	r.log.WithField("fetch", r.LastFetchDur).
		WithField("render", r.LastRenderDur).
		WithField("total", totalDur).
		Info("reconcile complete")

	r.reconcileLoopTime.Record(ctx, totalDur.Milliseconds())
	return results, nil
}
