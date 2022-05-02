package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type scheduleFunc func(ctx context.Context) error

type scheduledWorker struct {
	name     string
	fn       scheduleFunc
	interval time.Duration
}

type Scheduler struct {
	log     *logrus.Entry
	workers []*scheduledWorker
}

func (s *Scheduler) Register(name string, fn scheduleFunc, interval time.Duration) {
	for _, w := range s.workers {
		if w.name == name {
			panic(fmt.Sprintf("worker %s already registered", name))
		}
	}

	s.workers = append(s.workers, &scheduledWorker{
		name:     name,
		fn:       fn,
		interval: interval,
	})
}

func (s *Scheduler) Run(ctx context.Context) {
	for _, w := range s.workers {
		s.run(ctx, w)
	}
}

func (s *Scheduler) run(ctx context.Context, w *scheduledWorker) {
	for {
		if err := w.fn(ctx); err != nil {
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
