package reconciler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Reconciler struct {
	querier reconcilersql.Querier
	log     logrus.FieldLogger
	trigger chan struct{}

	// streamMu guards against concurrent streaming reconciles.
	streamMu sync.Mutex

	reconcileLoopTime metric.Int64Histogram
}

// DesiredState holds the results and phase durations of a reconcile run.
type DesiredState struct {
	Decisions  []DeployDecision
	FetchDur   time.Duration
	ComputeDur time.Duration
}

func New(pool *pgxpool.Pool, meter metric.Meter, log logrus.FieldLogger) (*Reconciler, error) {
	reconcileLoopTime, err := meter.Int64Histogram("reconciler_loop_duration", metric.WithDescription("Total time for one full reconcile loop"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile loop time histogram: %w", err)
	}

	return &Reconciler{
		querier:           reconcilersql.New(pool),
		log:               log,
		trigger:           make(chan struct{}, 1),
		reconcileLoopTime: reconcileLoopTime,
	}, nil
}

// Run starts the reconcile loop, writing results via the given writer.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration, dispatcher Dispatcher) {
	for {
		r.log.Info("reconciling")
		result, err := r.ComputeDesiredState(ctx)
		if err != nil {
			r.log.WithError(err).Error("reconcile")
		} else {
			ioStart := time.Now()
			if err := dispatcher.Dispatch(ctx, result.Decisions); err != nil {
				r.log.WithError(err).Error("dispatch")
			}
			r.log.WithField("io", time.Since(ioStart)).Info("decisions dispatched")
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

// ComputeDesiredState fetches the current state and computes deploy decisions for all
// environments. It performs no writes — callers decide what to do with the
// returned results (write to DB, display in UI, etc.).
func (r *Reconciler) ComputeDesiredState(ctx context.Context) (*DesiredState, error) {
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

	computeStart := time.Now()
	decisions := r.computeActions(snap)
	computeDur := time.Since(computeStart)

	deployCount := 0
	for _, d := range decisions {
		if d.Action == ActionDeploy {
			deployCount++
		}
	}
	r.log.WithField("total_decisions", len(decisions)).
		WithField("deploy_count", deployCount).
		Info("compute complete")

	totalDur := time.Since(loopStart)
	r.log.WithField("fetch", fetchDur).
		WithField("compute", computeDur).
		WithField("total", totalDur).
		Info("reconcile complete")

	r.reconcileLoopTime.Record(ctx, totalDur.Milliseconds())
	return &DesiredState{
		Decisions:  decisions,
		FetchDur:   fetchDur,
		ComputeDur: computeDur,
	}, nil
}

// StreamSummary is sent after all decisions have been streamed.
type StreamSummary struct {
	FetchDur   time.Duration `json:"fetchDur"`
	ComputeDur time.Duration `json:"computeDur"`
}

// StreamDecisions fetches the current state and streams deploy decisions to
// out, closing it when done. The caller must consume out (e.g. range over it).
// Returns a summary after all decisions are sent.
// Returns ErrReconcileInProgress if another stream is running.
func (r *Reconciler) StreamDecisions(ctx context.Context, out chan<- DeployDecision) (*StreamSummary, error) {
	if !r.streamMu.TryLock() {
		return nil, ErrReconcileInProgress
	}
	defer r.streamMu.Unlock()

	fetchStart := time.Now()
	snap, err := r.fetchSnapshot(ctx)
	if err != nil {
		close(out)
		return nil, fmt.Errorf("fetch snapshot: %w", err)
	}
	fetchDur := time.Since(fetchStart)

	computeStart := time.Now()
	r.computeActionsStream(snap, out) // closes out when done
	computeDur := time.Since(computeStart)

	return &StreamSummary{
		FetchDur:   fetchDur,
		ComputeDur: computeDur,
	}, nil
}
