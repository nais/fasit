package workers

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Schedulable interface {
	Run(ctx context.Context) error
}

type scheduledWorker struct {
	name     string
	v        Schedulable
	interval time.Duration
}

type Scheduler struct {
	log     *slog.Logger
	workers []*scheduledWorker
}

func NewScheduler(log *slog.Logger) *Scheduler {
	return &Scheduler{
		log: log.With("subsystem", "scheduler"),
	}
}

func (s *Scheduler) Register(name string, v Schedulable, interval time.Duration) {
	for _, w := range s.workers {
		if w.name == name {
			panic(fmt.Sprintf("worker %s already registered", name))
		}
	}

	s.workers = append(s.workers, &scheduledWorker{
		name:     name,
		v:        v,
		interval: interval,
	})
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, w := range s.workers {
		go s.run(ctx, w)
	}
}

func (s *Scheduler) run(ctx context.Context, w *scheduledWorker) {
	log := s.log.With("worker", w.name)
	for {
		log.Debug("running")
		if err := w.v.Run(ctx); err != nil {
			log.With("err", err).Error("error running scheduled worker")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.interval):
			// continue
		}
	}
}
