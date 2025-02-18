package workers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestScheduler(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	s := NewScheduler(logrus.NewEntry(logrus.StandardLogger()))

	tw1 := &testWorker{}
	tw2 := &testWorker{}
	s.Register("test_twice", tw1, 2*time.Second)
	s.Register("test_once", tw2, time.Minute)

	s.Start(ctx)

	<-ctx.Done()

	if tw1.NumRuns() != 2 {
		t.Errorf("expected test worker to run twice, but ran %v times", tw1.NumRuns())
	}
	if tw2.NumRuns() != 1 {
		t.Errorf("expected test worker to run once, but ran %v times", tw2.NumRuns())
	}
}

type testWorker struct {
	sync.Mutex
	runs int
}

func (t *testWorker) Run(ctx context.Context) error {
	t.Lock()
	defer t.Unlock()
	t.runs++
	return nil
}

func (t *testWorker) NumRuns() int {
	t.Lock()
	defer t.Unlock()
	return t.runs
}
