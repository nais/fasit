package workers

import (
	"context"
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

	if tw1.runs != 2 {
		t.Errorf("expected test worker to run twice, but ran %v times", tw1.runs)
	}
	if tw2.runs != 1 {
		t.Errorf("expected test worker to run once, but ran %v times", tw2.runs)
	}
}

type testWorker struct {
	runs int
}

func (t *testWorker) Run(ctx context.Context) error {
	t.runs++
	return nil
}
