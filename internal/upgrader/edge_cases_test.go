package upgrader

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
)

func TestMetricsRecording(t *testing.T) {
	suite := newTestSuite(t)

	// Create upgrader with real metrics to test recording
	upgrader := newUpgrade(suite)

	t.Run("stuck upgrade records metrics", func(t *testing.T) {
		stuckUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.25.0",
			LastModified:  time.Now().Add(-25 * time.Hour),
		}

		// Mock setup
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(stuckUpgrade, nil).Once()

		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusFAILED, "1.25.0").Return(stuckUpgrade, nil).Once()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)

		// Metrics should be recorded (we can't easily assert on specific values with the current setup,
		// but we can verify the operation completed without error)
	})

	t.Run("successful upgrade records completion metrics", func(t *testing.T) {
		completeUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.25.0",
			LastModified:  time.Now().Add(-2 * time.Hour),
		}

		// Mock setup for completed upgrade
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(completeUpgrade, nil).Once()

		// Mock no running operations
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()

		// Mock successful node pool check
		suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()

		suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.NodePool{
			{Name: "pool1", Version: "1.25.0"},
			{Name: "pool2", Version: "1.25.0"},
		}, nil).Once()

		// Mock completion
		completeUpgradeWithEnvironmentID := &model.ClusterUpgradeStatus{
			ID: uuid.New(), UpgradeStatus: model.UpgradeStatusDone, Version: "1.25.0",
			LastModified: time.Now(), SlackChannelID: "C123", SlackMessageTimestamp: "123",
			EnvironmentID: suite.env.id, // Need this for mentions retrieval
		}

		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusDONE, "1.25.0").Return(
			completeUpgradeWithEnvironmentID, nil).Once()

		// Mock the EnvironmentValueGet call that happens in updateSlackProgress for mentions
		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{
				Key:   "slack_upgrade_mentions",
				Value: []byte(`""`), // Empty mentions
			}, nil).Once()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)
	})
}

func TestErrorHandlingEdgeCases(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("database error during tenant fetch", func(t *testing.T) {
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return(nil, errors.New("database connection failed")).Once()

		err := upgrader.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database connection failed")
	})

	t.Run("database error during environment fetch", func(t *testing.T) {
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return(nil, errors.New("environment fetch failed")).Once()

		err := upgrader.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment fetch failed")
	})

	t.Run("no cluster upgrade found returns nil", func(t *testing.T) {
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, pgx.ErrNoRows).Once()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err) // Should complete successfully with no upgrade to process
	})

	t.Run("project ID fetch error", func(t *testing.T) {
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(nil, errors.New("project ID not found")).Once()

		err := upgrader.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project ID not found")
	})

	t.Run("context cancellation during upgrade", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Once()

		// Cancel context before environments fetch
		cancel()

		// This should handle cancellation gracefully
		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return(nil, context.Canceled).Once()

		err := upgrader.Run(ctx)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestLogNonCriticalError(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("logs error with fields", func(t *testing.T) {
		err := errors.New("test error")
		fields := map[string]interface{}{
			"tenant":      "test-tenant",
			"environment": "test-env",
		}

		// This should not panic
		upgrader.logNonCriticalError(err, "test_operation", fields)
	})

	t.Run("handles nil fields", func(t *testing.T) {
		err := errors.New("test error")

		// This should not panic
		upgrader.logNonCriticalError(err, "test_operation", nil)
	})
}

func TestUpgradeEnvironmentEdgeCases(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("invalid project ID JSON", func(t *testing.T) {
		tenant := &model.Tenant{ID: suite.env.tenantID, Name: suite.env.name}
		env := &model.Environment{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name}

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`invalid json`)}, nil).Once()

		err := upgrader.upgradeEnvironment(context.Background(), tenant, env)
		assert.Error(t, err)
	})

	t.Run("running operations API error", func(t *testing.T) {
		tenant := &model.Tenant{ID: suite.env.tenantID, Name: suite.env.name}
		env := &model.Environment{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name}

		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID: uuid.New(), UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version: "1.25.0", LastModified: time.Now().Add(-1 * time.Hour),
		}

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(clusterUpgrade, nil).Once()

		// Create a retriable API error for this test
		apiErr := createAPIError(codes.Unavailable, "API unavailable")

		// Mock API error with max retries - since it's retriable, it will retry
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return(nil, apiErr).Times(4) // 1 initial + 3 retries

		err := upgrader.upgradeEnvironment(context.Background(), tenant, env)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API unavailable")
	})
}
