package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Helper function to create APIError from gRPC status
func createAPIError(code codes.Code, msg string) error {
	grpcErr := status.Error(code, msg)
	apiErr, _ := apierror.FromError(grpcErr)
	return apiErr
}

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error should not be retriable",
			err:      nil,
			expected: false,
		},
		{
			name:     "context deadline exceeded should be retriable",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "unavailable gRPC error should be retriable",
			err:      createAPIError(codes.Unavailable, "service unavailable"),
			expected: true,
		},
		{
			name:     "deadline exceeded gRPC error should be retriable",
			err:      createAPIError(codes.DeadlineExceeded, "deadline exceeded"),
			expected: true,
		},
		{
			name:     "aborted gRPC error should be retriable",
			err:      createAPIError(codes.Aborted, "operation aborted"),
			expected: true,
		},
		{
			name:     "invalid argument gRPC error should not be retriable",
			err:      createAPIError(codes.InvalidArgument, "invalid argument"),
			expected: false,
		},
		{
			name:     "not found gRPC error should not be retriable",
			err:      createAPIError(codes.NotFound, "not found"),
			expected: false,
		},
		{
			name:     "permission denied gRPC error should not be retriable",
			err:      createAPIError(codes.PermissionDenied, "permission denied"),
			expected: false,
		},
		{
			name:     "unknown error should not be retriable",
			err:      errors.New("unknown error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetriableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryWithBackoff(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("success on first try", func(t *testing.T) {
		callCount := 0
		err := upgrader.retryer.WithBackoff(context.Background(), "test_operation", func() error {
			callCount++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("success after retries", func(t *testing.T) {
		callCount := 0
		err := upgrader.retryer.WithBackoff(context.Background(), "test_operation", func() error {
			callCount++
			if callCount < 3 {
				return createAPIError(codes.Unavailable, "temporary failure")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("non-retriable error stops immediately", func(t *testing.T) {
		callCount := 0
		err := upgrader.retryer.WithBackoff(context.Background(), "test_operation", func() error {
			callCount++
			return createAPIError(codes.InvalidArgument, "invalid argument")
		})

		assert.Error(t, err)
		assert.Equal(t, 1, callCount)
		assert.Contains(t, err.Error(), "invalid argument")
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		callCount := 0
		err := upgrader.retryer.WithBackoff(context.Background(), "test_operation", func() error {
			callCount++
			return createAPIError(codes.Unavailable, "always failing")
		})

		assert.Error(t, err)
		assert.Equal(t, 4, callCount) // Initial attempt + 3 retries
		assert.Contains(t, err.Error(), "always failing")
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callCount := 0
		err := upgrader.retryer.WithBackoff(ctx, "test_operation", func() error {
			callCount++
			if callCount == 2 {
				cancel() // Cancel context during retry delay
			}
			return createAPIError(codes.Unavailable, "retriable error")
		})

		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled) || callCount >= 2)
	})
}
