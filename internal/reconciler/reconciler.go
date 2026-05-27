package reconciler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log logrus.FieldLogger) Publisher

type Reconciler struct {
	querier      reconcilersql.Querier
	newPublisher NewPublisher
	log          logrus.FieldLogger
	trigger      chan struct{}

	publishersMu sync.Mutex
	publishers   map[string]Publisher

	reconcileTime     metric.Int64Histogram
	reconcileLoopTime metric.Int64Histogram
	deployMessages    metric.Int64Counter

	// Phase durations from the last Reconcile call.
	LastFetchDur  time.Duration
	LastRenderDur time.Duration
	LastIODur     time.Duration
}

func New(querier reconcilersql.Querier, publisher NewPublisher, meter metric.Meter, log logrus.FieldLogger) (*Reconciler, error) {
	reconcileTime, err := meter.Int64Histogram("reconciler_environment_time", metric.WithDescription("Time spent reconciling per environment"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile time histogram: %w", err)
	}
	reconcileLoopTime, err := meter.Int64Histogram("reconciler_loop_duration", metric.WithDescription("Total time for one full reconcile loop"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create reconcile loop time histogram: %w", err)
	}
	deployMessages, err := meter.Int64Counter("reconciler_deploy_messages", metric.WithDescription("Deploy messages sent by reconciler"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}

	return &Reconciler{
		querier:           querier,
		newPublisher:      publisher,
		log:               log,
		trigger:           make(chan struct{}, 1),
		publishers:        make(map[string]Publisher),
		reconcileTime:     reconcileTime,
		reconcileLoopTime: reconcileLoopTime,
		deployMessages:    deployMessages,
	}, nil
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	for {
		r.log.Info("reconciling")
		if err := r.Reconcile(ctx); err != nil {
			r.log.WithError(err).Error("reconcile")
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

func (r *Reconciler) Reconcile(ctx context.Context) error {
	loopStart := time.Now()

	fetchStart := time.Now()
	snap, err := r.fetchSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("fetch snapshot: %w", err)
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
		if res.Action == actionDeploy {
			deployCount++
		}
	}
	r.log.WithField("total_results", len(results)).
		WithField("deploy_count", deployCount).
		Info("render complete")

	ioStart := time.Now()
	if err := r.writeResults(ctx, results); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	ioDur := time.Since(ioStart)

	r.LastFetchDur = fetchDur
	r.LastRenderDur = renderDur
	r.LastIODur = ioDur

	totalDur := time.Since(loopStart)
	r.log.WithField("fetch", fetchDur).
		WithField("render", renderDur).
		WithField("io", ioDur).
		WithField("total", totalDur).
		Info("reconcile loop complete")

	r.reconcileLoopTime.Record(ctx, totalDur.Milliseconds())
	return nil
}

func (r *Reconciler) publisher(topicID string) Publisher {
	r.publishersMu.Lock()
	defer r.publishersMu.Unlock()
	if p, ok := r.publishers[topicID]; ok {
		return p
	}
	p := r.newPublisher(topicID, r.log)
	r.publishers[topicID] = p
	return p
}
