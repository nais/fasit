package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

func newRetryerForTest(config RetryConfig) *Retryer {
	log := logrus.New().WithField("testSuite", "retry")
	meter := metricsdk.NewMeterProvider().Meter("retry-test")
	gkeAPICalls, _ := meter.Int64Counter("test_gke_api_calls")
	gkeAPIErrors, _ := meter.Int64Counter("test_gke_api_errors")
	retryAttempts, _ := meter.Int64Counter("test_retry_attempts")
	return NewRetryer(log, gkeAPICalls, gkeAPIErrors, retryAttempts, config)
}

func TestIsRetriableError(t *testing.T) {
	if isRetriableError(nil) {
		t.Fatalf("expected nil error to be non-retriable")
	}

	if !isRetriableError(context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded to be retriable")
	}

	if isRetriableError(errors.New("boom")) {
		t.Fatalf("expected generic error to be non-retriable")
	}
}

func TestRetryerWithBackoff_NonRetriableDoesNotRetry(t *testing.T) {
	retryer := newRetryerForTest(RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 2 * time.Millisecond})

	wantErr := errors.New("non-retriable")
	attempts := 0
	err := retryer.WithBackoff(context.Background(), "non_retriable_test", func() error {
		attempts++
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}

	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetryerWithBackoff_RetriableEventuallySucceeds(t *testing.T) {
	retryer := newRetryerForTest(RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 2 * time.Millisecond})

	attempts := 0
	err := retryer.WithBackoff(context.Background(), "retriable_then_success", func() error {
		attempts++
		if attempts < 3 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryerWithBackoff_RetriableExhausted(t *testing.T) {
	retryer := newRetryerForTest(RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 2 * time.Millisecond})

	attempts := 0
	err := retryer.WithBackoff(context.Background(), "retriable_exhausted", func() error {
		attempts++
		return context.DeadlineExceeded
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryerWithBackoff_ContextCanceledDuringBackoff(t *testing.T) {
	retryer := newRetryerForTest(RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 10 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	err := retryer.WithBackoff(ctx, "canceled_context", func() error {
		attempts++
		return context.DeadlineExceeded
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if attempts != 1 {
		t.Fatalf("expected 1 attempt before cancellation, got %d", attempts)
	}
}
