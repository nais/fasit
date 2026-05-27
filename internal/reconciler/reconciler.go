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

	reconcileLoopTime metric.Int64Histogram
}

// ReconcileResult holds the results and phase durations of a reconcile run.
type ReconcileResult struct {
	Results   []Result
	FetchDur  time.Duration
	RenderDur time.Duration
}

func New(querier reconcilersql.Querier, meter metric.Meter, log logrus.FieldLogger) (*Reconciler, error) {
	reconcileLoopTime, err := meter.Int64Histogram("reconciler_loop_duration", metric.WithDescription("Total time for one full reconcile loop"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile loop time histogram: %w", err)
	}

	return &Reconciler{
		querier:           querier,
		log:               log,
		trigger:           make(chan struct{}, 1),
		reconcileLoopTime: reconcileLoopTime,
	}, nil
}

// Run starts the reconcile loop, writing results via the given writer.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration, writer ResultWriter) {
	for {
		r.log.Info("reconciling")
		result, err := r.Reconcile(ctx)
		if err != nil {
			r.log.WithError(err).Error("reconcile")
		} else {
			ioStart := time.Now()
			if err := writer.WriteResults(ctx, result.Results); err != nil {
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
func (r *Reconciler) Reconcile(ctx context.Context) (*ReconcileResult, error) {
	loopStart := time.Now()

	fetchStart := time.Now()
	snap, err := r.fetchSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch snapshot: %w", err)
	}
	fetchDur := time.Since(fetchStart)

	r.log.WithField("num_envs", len(snap.environments)).
		WithField("num_deployments", len(snap.deployments)).
		Info("reconciling tenant environments")

	renderStart := time.Now()
	results := r.renderAll(snap)
	renderDur := time.Since(renderStart)

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
	r.log.WithField("fetch", fetchDur).
		WithField("render", renderDur).
		WithField("total", totalDur).
		Info("reconcile complete")

	r.reconcileLoopTime.Record(ctx, totalDur.Milliseconds())
	return &ReconcileResult{
		Results:   results,
		FetchDur:  fetchDur,
		RenderDur: renderDur,
	}, nil
}
