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
		}

		// Mock tenant and environment setup
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID:   suite.env.tenantID,
			Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID:       suite.env.id,
			TenantID: suite.env.tenantID,
			Name:     suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(stuckUpgrade, nil).Once()

		// Expect stuck upgrade to be marked as failed
		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusFAILED, "1.25.0").Return(stuckUpgrade, nil).Once()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)
	})

	t.Run("stuck upgrade without slack metadata uses fallback", func(t *testing.T) {
		// Setup stuck cluster upgrade without Slack metadata
		stuckUpgrade := &model.ClusterUpgradeStatus{
			ID:                    uuid.New(),
			UpgradeStatus:         model.UpgradeStatusNodeUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now().Add(-26 * time.Hour),
			StartTime:             time.Now().Add(-26 * time.Hour),
			SlackChannelID:        "", // No existing slack message
			SlackMessageTimestamp: "",
		}

		// Mock setup
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID:   suite.env.tenantID,
			Name: suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID:       suite.env.id,
			TenantID: suite.env.tenantID,
			Name:     suite.env.name,
		}}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(stuckUpgrade, nil).Once()

		// Expect stuck upgrade to be marked as failed
		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusFAILED, "1.25.0").Return(stuckUpgrade, nil).Once()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)
	})

	t.Run("not stuck upgrade continues normally", func(t *testing.T) {
		// Create fresh upgrader for this test
		testSuite := newTestSuite(t)
		upgrader := newUpgrade(testSuite)

		// Setup recent upgrade that's not stuck (only 2 hours old)
		notStuckUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.25.0",
			LastModified:  time.Now().Add(-2 * time.Hour), // Only 2 hours old
			StartTime:     time.Now().Add(-2 * time.Hour),
		}

		// Setup mocks for this test
		testSuite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID:   testSuite.env.tenantID,
			Name: testSuite.env.name,
		}}, nil).Once()

		testSuite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, testSuite.env.tenantID).Return([]*model.Environment{{
			ID:       testSuite.env.id,
			TenantID: testSuite.env.tenantID,
			Name:     testSuite.env.name,
		}}, nil).Once()

		testSuite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Once()

		testSuite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(notStuckUpgrade, nil).Once()

		// System will first check for running operations
		testSuite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Once()

		// Mock the master upgrade status check
		testSuite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(
			&model.EnvironmentOperation{
				ID:     uuid.New(),
				Name:   "operation",
				Status: "RUNNING",
				Type:   "UPGRADE_MASTER",
			}, nil).Once()

		testSuite.upgradeMock.EXPECT().GetOperation(mock.Anything, mock.Anything, "operation").Return(
			&containerpb.Operation{Status: containerpb.Operation_RUNNING}, nil).Once()
		testSuite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()

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
		}}, nil).Once()

		testSuite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, testSuite.env.tenantID).Return([]*model.Environment{{
			ID:       testSuite.env.id,
			TenantID: testSuite.env.tenantID,
			Name:     testSuite.env.name,
		}}, nil).Once()

		testSuite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{
				Key:   "project_id",
				Value: []byte(`"1234"`),
			}, nil).Once()

		testSuite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, testSuite.env.tenantID, testSuite.env.id).Return(doneUpgrade, nil).Once()

		// Even for done upgrades, the system will check for running operations
		testSuite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Once()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err)

		// Verify no stuck upgrade handling was called
		testSuite.repoMock.AssertNotCalled(t, "UpdateClusterUpgradeStatus", mock.Anything, mock.Anything, mock.Anything, gensql.ClusterUpgradesStatusFAILED, mock.Anything)
	})
}
