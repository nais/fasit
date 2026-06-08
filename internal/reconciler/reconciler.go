package reconciler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"go.opentelemetry.io/otel/metric"
)

var trigger chan struct{}

func init() {
	trigger = make(chan struct{}, 1)
}

type Reconciler struct {
	querier reconcilersql.Querier
	log     *slog.Logger

	// streamMu guards against concurrent streaming reconciles.
	streamMu sync.Mutex

	reconcileLoopTime metric.Int64Histogram
}

// DesiredState holds the results and phase durations of a reconcile run.
type DesiredState struct {
	Results    []ComputeResult
	FetchDur   time.Duration
	ComputeDur time.Duration
}

func New(pool *pgxpool.Pool, meter metric.Meter, log *slog.Logger) (*Reconciler, error) {
	reconcileLoopTime, err := meter.Int64Histogram("reconciler_loop_duration", metric.WithDescription("Total time for one full reconcile loop"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile loop time histogram: %w", err)
	}

	return &Reconciler{
		querier:           reconcilersql.New(pool),
		log:               log,
		reconcileLoopTime: reconcileLoopTime,
	}, nil
}

// Run starts the reconcile loop, writing results via the given writer.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration, deployer Deployer) {
	for {
		r.log.Info("reconciling")
		desiredState, err := r.ComputeDesiredState(ctx)
		if err != nil {
			r.log.With("err", err).Error("reconcile")
		} else {
			ioStart := time.Now()
			if err := r.writeResultLogs(ctx, desiredState.Results); err != nil {
				r.log.With("err", err).Error("log compute results")
			} else if err := deployer.Deploy(ctx, desiredState.Results); err != nil {
				r.log.With("err", err).Error("deploy")
			}
			r.log.With("io", time.Since(ioStart)).Info("decisions deployed")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		case <-trigger:
			r.log.Info("manual reconcile triggered")
		}
	}
}

// writeResultLogs appends a decision_log row for every (environment, feature)
// whose decision changed since the last cycle. Change detection compares the
// feature assignment, version, action, and message against the latest decision.
func (r *Reconciler) writeResultLogs(ctx context.Context, results []ComputeResult) error {
	latest, err := r.querier.ListLatestDecisions(ctx)
	if err != nil {
		return fmt.Errorf("list latest decisions: %w", err)
	}

	type key struct {
		env  uuid.UUID
		feat string
	}
	prev := make(map[key]reconcilersql.ListLatestDecisionsRow, len(latest))
	for _, d := range latest {
		prev[key{d.EnvironmentID, d.FeatureName}] = d
	}

	var changed []reconcilersql.AppendDecisionsParams
	for _, res := range results {
		p, ok := prev[key{res.EnvironmentID, res.Feature.Name}]
		if ok &&
			p.FeatureAssignmentID == res.FeatureAssignmentID &&
			p.FeatureVersion == res.Feature.Version &&
			p.Action == res.Action.String() &&
			p.Message == res.Message {
			continue
		}
		changed = append(changed, reconcilersql.AppendDecisionsParams{
			EnvironmentID:       res.EnvironmentID,
			FeatureAssignmentID: res.FeatureAssignmentID,
			FeatureName:         res.Feature.Name,
			FeatureVersion:      res.Feature.Version,
			Action:              res.Action.String(),
			Message:             res.Message,
		})
	}

	if len(changed) > 0 {
		if _, err := r.querier.AppendDecisions(ctx, changed); err != nil {
			return fmt.Errorf("append decisions: %w", err)
		}
	}
	return nil
}

// TimeoutDeployInstructions periodically marks deploys stuck in pending for more
// than one hour as failed by appending a terminal deploy_log row.
func (r *Reconciler) TimeoutDeployInstructions(ctx context.Context, log *slog.Logger) {
	for {
		err := r.querier.TimeoutPendingDeploys(ctx)
		if err != nil {
			log.With("err", err).Error("failed to timeout deploy instructions")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Minute):
		}
	}
}

func TriggerReconcile() {
	select {
	case trigger <- struct{}{}:
	default:
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

	r.log.With("num_envs", len(snap.environments),
		"num_assignments", len(snap.assignments)).Info("reconciling tenant environments")

	computeStart := time.Now()
	decisions := r.computeActions(snap)
	computeDur := time.Since(computeStart)

	deployCount := 0
	for _, d := range decisions {
		if d.Action == ActionDeploy {
			deployCount++
		}
	}
	r.log.With("total_decisions", len(decisions),
		"deploy_count", deployCount).Info("compute complete")

	totalDur := time.Since(loopStart)
	r.log.With("fetch", fetchDur,
		"compute", computeDur,
		"total", totalDur).Info("reconcile complete")

	r.reconcileLoopTime.Record(ctx, totalDur.Milliseconds())
	return &DesiredState{
		Results:    decisions,
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
func (r *Reconciler) StreamDecisions(ctx context.Context, out chan<- ComputeResult) (*StreamSummary, error) {
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
