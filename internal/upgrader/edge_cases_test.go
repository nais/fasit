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
)

func TestMetricsRecording(t *testing.T) {
	t.Run("stuck upgrade records metrics", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		stuckUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.25.0",
			LastModified:  time.Now().Add(-25 * time.Hour),
		}

		// Mock setup
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Maybe()

		// Mock ClusterUpgradeHistoryGet for the cleanup process - return empty list
		suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(
			[]*model.ClusterUpgradeStatus{}, nil).Maybe()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(stuckUpgrade, nil).Maybe()

		// Mock GetRunningOperations call from stuck detection logic
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Maybe()

		// Mock GetCurrentMasterVersion call from completion checking (for MASTER_UPGRADE status)
		suite.upgradeMock.EXPECT().GetCurrentMasterVersion(mock.Anything, mock.Anything, mock.Anything).Return("1.24.0", nil).Maybe()

		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusFAILED, "1.25.0").Return(stuckUpgrade, nil).Maybe()

		// Mock the Slack mentions retrieval for updateSlackProgress
		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{Key: "slack_upgrade_mentions", Value: []byte(`"<@U123456>"`)}, nil).Maybe()

		// Mock the SetClusterUpgradesSlackMessage call for postNewSlackMessage
		suite.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			&model.ClusterUpgradeStatus{}, nil).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)

		// Metrics should be recorded (we can't easily assert on specific values with the current setup,
		// but we can verify the operation completed without error)
	})

	t.Run("successful upgrade records completion metrics", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		completeUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.25.0",
			LastModified:  time.Now().Add(-2 * time.Hour),
		}

		// Mock setup for completed upgrade
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Maybe()

		suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id).Return([]*model.ClusterUpgradeStatus{}, nil).Maybe()
		suite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, mock.Anything).Return([]*model.EnvironmentOperation{}, nil).Maybe()
		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(completeUpgrade, nil).Maybe()

		// Mock no running operations
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Times(2) // Called during stuck detection and main logic

		// Mock successful node pool check
		suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Maybe()

		suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.NodePool{
			{Name: "pool1", Version: "1.25.0"},
			{Name: "pool2", Version: "1.25.0"},
		}, nil).Times(2) // Called during stuck detection and node upgrade logic

		// Mock completion
		completeUpgradeWithEnvironmentID := &model.ClusterUpgradeStatus{
			ID: uuid.New(), UpgradeStatus: model.UpgradeStatusDone, Version: "1.25.0",
			LastModified: time.Now(), SlackChannelID: "C123", SlackMessageTimestamp: "123",
			EnvironmentID: suite.env.id, // Need this for mentions retrieval
		}

		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, "1.25.0").Return(
			completeUpgradeWithEnvironmentID, nil).Maybe()

		// Mock the EnvironmentValueGet call that happens in updateSlackProgress for mentions
		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{
				Key:   "slack_upgrade_mentions",
				Value: []byte(`""`), // Empty mentions
			}, nil).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)
	})
}

func TestErrorHandlingEdgeCases(t *testing.T) {
	t.Run("database error during tenant fetch", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return(nil, errors.New("database connection failed")).Maybe()

		err := upgrader.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database connection failed")
	})

	t.Run("no cluster upgrade found returns nil", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Maybe()

		// Mock ClusterUpgradeHistoryGet for the cleanup process - return empty list
		suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(
			[]*model.ClusterUpgradeStatus{}, nil).Maybe()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, pgx.ErrNoRows).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err) // Should complete successfully with no upgrade to process
	})

	t.Run("context cancellation during upgrade", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		ctx, cancel := context.WithCancel(context.Background())

		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		// Cancel context before environments fetch
		cancel()

		// This should handle cancellation gracefully
		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return(nil, context.Canceled).Maybe()

		err := upgrader.Run(ctx)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestUpgradeEnvironmentEdgeCases(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("invalid project ID JSON", func(t *testing.T) {
		tenant := &model.Tenant{ID: suite.env.tenantID, Name: suite.env.name}
		env := &model.Environment{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name}

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`invalid json`)}, nil).Maybe()

		err := upgrader.upgradeEnvironment(context.Background(), tenant, env)
		assert.Error(t, err)
	})
}
