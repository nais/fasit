package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
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
	log     *logrus.Entry
	workers []*scheduledWorker
}

func NewScheduler(log *logrus.Entry) *Scheduler {
	return &Scheduler{
		log: log,
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
	for {
		if err := w.v.Run(ctx); err != nil {
			s.log.WithError(err).WithField("worker", w.name).Error("error running scheduled worker")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.interval):
			// continue
		}
	}
}
