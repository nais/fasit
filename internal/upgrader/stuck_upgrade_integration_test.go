package upgrader

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStuckUpgradeIntegration(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("stuck upgrade detected and handled", func(t *testing.T) {
		// Setup stuck cluster upgrade (older than 24 hours)
		stuckUpgrade := &model.ClusterUpgradeStatus{
			ID:                    uuid.New(),
			UpgradeStatus:         model.UpgradeStatusMasterUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now().Add(-25 * time.Hour),
			StartTime:             time.Now().Add(-25 * time.Hour),
			SlackChannelID:        "C123456",
			SlackMessageTimestamp: "1234567890.123456",
			EnvironmentID:         suite.env.id, // Need this for mentions retrieval
		}

		// Mock tenant and environment setup
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID:   suite.env.tenantID,
			Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID:       suite.env.id,
			TenantID: suite.env.tenantID,
			Name:     suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Maybe()

		// Mock ClusterUpgradeHistoryGet for the cleanup process - return empty list
		suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(
			[]*model.ClusterUpgradeStatus{}, nil).Maybe()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(stuckUpgrade, nil).Maybe()

		// Mock GetRunningOperations call from stuck detection logic
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Maybe()

		// Mock GetCurrentMasterVersion call from completion checking (for MASTER_UPGRADE status)
		suite.upgradeMock.EXPECT().GetCurrentMasterVersion(mock.Anything, mock.Anything, mock.Anything).Return("1.24.0", nil).Maybe()

		// Mock the EnvironmentValueGet call that happens in updateSlackProgress for mentions
		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{
				Key:   "slack_upgrade_mentions",
				Value: []byte(`""`), // Empty mentions
			}, nil).Maybe()

		// Expect stuck upgrade to be marked as failed
		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusFAILED, "1.25.0").Return(stuckUpgrade, nil).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)
	})

	t.Run("not stuck upgrade continues normally", func(t *testing.T) {
		// Create fresh upgrader for this test
		testSuite := newTestSuite(t)
		upgrader := newUpgrade(testSuite)

		// Setup recent upgrade that's not stuck (only 2 hours old)
		notStuckUpgrade := &model.ClusterUpgradeStatus{
			ID:                    uuid.New(),
			UpgradeStatus:         model.UpgradeStatusMasterUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now().Add(-2 * time.Hour), // Only 2 hours old
			StartTime:             time.Now().Add(-2 * time.Hour),
			EnvironmentID:         testSuite.env.id, // Need this for Slack mentions retrieval
			SlackChannelID:        "C123456",        // Existing Slack message
			SlackMessageTimestamp: "1234567890.123456",
		}

		// Setup mocks for this test
		testSuite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID:   testSuite.env.tenantID,
			Name: testSuite.env.name,
		}}, nil).Maybe()

		testSuite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, testSuite.env.tenantID).Return([]*model.Environment{{
			ID:       testSuite.env.id,
			TenantID: testSuite.env.tenantID,
			Name:     testSuite.env.name,
		}}, nil).Maybe()

		testSuite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Maybe()

		// Mock ClusterUpgradeHistoryGet for the cleanup process - return empty list
		testSuite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(
			[]*model.ClusterUpgradeStatus{}, nil).Maybe()

		// Mock ClusterOperationsGetByUpgradeID for cleanup process
		testSuite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, mock.Anything).Return(
			[]*model.EnvironmentOperation{}, nil).Maybe()

		testSuite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(notStuckUpgrade, nil).Maybe()

		// System will first check for running operations
		testSuite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Times(2) // Called during stuck detection and main logic

		// Mock GetCurrentMasterVersion call from completion checking (for MASTER_UPGRADE status)
		testSuite.upgradeMock.EXPECT().GetCurrentMasterVersion(mock.Anything, mock.Anything, mock.Anything).Return("1.25.0", nil).Maybe() // Return target version to indicate completion

		// Mock the master upgrade status check
		testSuite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(
			&model.EnvironmentOperation{
				ID:     uuid.New(),
				Name:   "operation",
				Status: "RUNNING",
				Type:   "UPGRADE_MASTER",
			}, nil).Maybe()

		testSuite.upgradeMock.EXPECT().GetOperation(mock.Anything, mock.Anything, "operation").Return(
			&containerpb.Operation{Status: containerpb.Operation_RUNNING}, nil).Maybe()
		testSuite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		// Mock UpdateClusterUpgradeStatus for normal status progression but not FAILED
		testSuite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		// Mock SetClusterUpgradesSlackMessage for Slack updates
		testSuite.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(notStuckUpgrade, nil).Maybe()

		// Mock the EnvironmentValueGet call for Slack mentions that happens in updateSlackProgress
		testSuite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{
				Key:   "slack_upgrade_mentions",
				Value: []byte(`""`), // Empty mentions
			}, nil).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)

		// Verify UpdateClusterUpgradeStatus was NOT called with FAILED status
		testSuite.repoMock.AssertNotCalled(t, "UpdateClusterUpgradeStatus", mock.Anything, mock.Anything, mock.Anything, gensql.ClusterUpgradesStatusFAILED, mock.Anything)
	})

	t.Run("done upgrade is not considered stuck", func(t *testing.T) {
		// Create fresh upgrader for this test
		testSuite := newTestSuite(t)
		upgrader := newUpgrade(testSuite)

		// Setup old but completed upgrade
		doneUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusDone, // Completed
			Version:       "1.25.0",
			LastModified:  time.Now().Add(-48 * time.Hour), // Very old
			StartTime:     time.Now().Add(-48 * time.Hour),
		}

		// Mock setup
		testSuite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID:   testSuite.env.tenantID,
			Name: testSuite.env.name,
		}}, nil).Maybe()

		testSuite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, testSuite.env.tenantID).Return([]*model.Environment{{
			ID:       testSuite.env.id,
			TenantID: testSuite.env.tenantID,
			Name:     testSuite.env.name,
		}}, nil).Maybe()

		testSuite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Maybe()

		// Mock ClusterUpgradeHistoryGet for the cleanup process - return the done upgrade for cleanup
		testSuite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(
			[]*model.ClusterUpgradeStatus{doneUpgrade}, nil).Maybe()

		// Mock ClusterOperationsGetByUpgradeID for the cleanup process - return empty operations
		testSuite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, doneUpgrade.ID).Return(
			[]*model.EnvironmentOperation{}, nil).Maybe()

		testSuite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(doneUpgrade, nil).Maybe()

		// Even for done upgrades, the system will check for running operations
		testSuite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)

		// Verify no stuck upgrade handling was called
		testSuite.repoMock.AssertNotCalled(t, "UpdateClusterUpgradeStatus", mock.Anything, mock.Anything, mock.Anything, gensql.ClusterUpgradesStatusFAILED, mock.Anything)
	})
}
