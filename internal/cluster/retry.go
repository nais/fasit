package cluster

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig returns the standard retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// Retryer provides retry functionality with metrics and logging
type Retryer struct {
	log           logrus.FieldLogger
	gkeAPICalls   metric.Int64Counter
	gkeAPIErrors  metric.Int64Counter
	retryAttempts metric.Int64Counter
	config        RetryConfig
}

// NewRetryer creates a new Retryer with the given metrics and configuration
func NewRetryer(log logrus.FieldLogger, gkeAPICalls, gkeAPIErrors, retryAttempts metric.Int64Counter, config RetryConfig) *Retryer {
	return &Retryer{
		log:           log,
		gkeAPICalls:   gkeAPICalls,
		gkeAPIErrors:  gkeAPIErrors,
		retryAttempts: retryAttempts,
		config:        config,
	}
}

// WithBackoff executes a function with exponential backoff for retriable errors
func (r *Retryer) WithBackoff(ctx context.Context, operation string, fn func() error) error {
	// Record API call metric
	r.gkeAPICalls.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", operation)))

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 0 {
				r.log.WithFields(logrus.Fields{
					"operation":      operation,
					"attempt":        attempt + 1,
					"total_attempts": attempt + 1,
					"success":        true,
				}).Info("operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Record API error metric
		r.gkeAPIErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.Bool("retriable", isRetriableError(err)),
		))

		// Don't retry non-retriable errors
		if !isRetriableError(err) {
			r.log.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt + 1,
				"retriable": false,
				"error":     err.Error(),
			}).Warn("non-retriable error, not retrying")
			return err
		}

		// Don't retry on last attempt
		if attempt == r.config.MaxRetries {
			break
		}

		// Record retry attempt metric
		r.retryAttempts.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.Int("attempt", attempt+1),
		))

		// Calculate delay with exponential backoff
		delay := time.Duration(float64(r.config.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > r.config.MaxDelay {
			delay = r.config.MaxDelay
		}

		r.log.WithFields(logrus.Fields{
			"operation":    operation,
			"attempt":      attempt + 1,
			"next_attempt": attempt + 2,
			"delay":        delay.String(),
			"error":        err.Error(),
		}).Warn("retriable error, retrying after delay")

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	r.log.WithFields(logrus.Fields{
		"operation":      operation,
		"max_attempts":   r.config.MaxRetries + 1,
		"total_attempts": r.config.MaxRetries + 1,
		"final_error":    lastErr.Error(),
	}).Error("operation failed after all retry attempts")

	return lastErr
}

// isRetriableError checks if an error should be retried
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a GKE API error
	if apiErr, ok := err.(*apierror.APIError); ok {
		// Retriable errors: rate limits, temporary unavailable, etc.
		switch apiErr.GRPCStatus().Code() {
		case codes.Unavailable, codes.DeadlineExceeded, codes.Aborted:
			return true
		case codes.InvalidArgument, codes.NotFound, codes.PermissionDenied:
			return false // Permanent errors
		}
	}

	// Database connection errors are typically retriable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Default to non-retriable for safety
	return false
}
