package upgrader

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func TestSlackNotificationFlow(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("updateSlackProgress handles missing message metadata gracefully", func(t *testing.T) {
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:                    uuid.New(),
			UpgradeStatus:         model.UpgradeStatusControlPlaneUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now(),
			SlackChannelID:        "",         // No channel ID
			SlackMessageTimestamp: "",         // No timestamp
			EnvironmentID:         uuid.New(), // Need this for the fallback
		}

		// Mock the EnvironmentValueGet call that happens in getUpgradeMentions during fallback
		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, clusterUpgrade.EnvironmentID, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{
				Key:   "slack_upgrade_mentions",
				Value: []byte(`""`), // Empty mentions
			}, nil).Once()

		// Mock the SetClusterUpgradesSlackMessage call that happens when fallback posts a new message
		suite.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, clusterUpgrade.ID, "", "").Return(
			&model.ClusterUpgradeStatus{}, nil).Once()
		// This should not cause any panic or error - should post a new message via fallback
		upgrader.updateSlackProgress(context.Background(), "tenant1", "env1", clusterUpgrade)

		// Test passes if no panic occurs
	})

	t.Run("updateSlackProgress with valid message metadata", func(t *testing.T) {
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:                    uuid.New(),
			UpgradeStatus:         model.UpgradeStatusControlPlaneUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now(),
			SlackChannelID:        "C123456",
			SlackMessageTimestamp: "1234567890.123456",
			EnvironmentID:         uuid.New(), // Need this for mentions retrieval
		}

		// Mock the EnvironmentValueGet call that now happens in updateSlackProgress
		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, clusterUpgrade.EnvironmentID, "slack_upgrade_mentions", false).Return(
			&model.EnvironmentValue{
				Key:   "slack_upgrade_mentions",
				Value: []byte(`"<@U123456>"`), // Some test mentions
			}, nil).Once()
		// This should attempt to update the message
		upgrader.updateSlackProgress(context.Background(), "tenant1", "env1", clusterUpgrade)

		// Test passes if no panic occurs (fake client returns error which is logged)
	})
}

func TestSlackProgressStates(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	testCases := []struct {
		name   string
		status model.UpgradeStatus
	}{
		{
			name:   "master upgrade status",
			status: model.UpgradeStatusControlPlaneUpgrade,
		},
		{
			name:   "node upgrade status",
			status: model.UpgradeStatusNodeUpgrade,
		},
		{
			name:   "completed status",
			status: model.UpgradeStatusDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clusterUpgrade := &model.ClusterUpgradeStatus{
				ID:                    uuid.New(),
				UpgradeStatus:         tc.status,
				Version:               "1.25.0",
				LastModified:          time.Now(),
				SlackChannelID:        "C123456",
				SlackMessageTimestamp: "1234567890.123456",
				EnvironmentID:         uuid.New(),
			}

			// Mock the EnvironmentValueGet call that now happens in updateSlackProgress
			suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, clusterUpgrade.EnvironmentID, "slack_upgrade_mentions", false).Return(
				&model.EnvironmentValue{
					Key:   "slack_upgrade_mentions",
					Value: []byte(`"<@U123456>"`),
				}, nil).Once()
			// Test that the function doesn't panic
			upgrader.updateSlackProgress(context.Background(), "tenant1", "env1", clusterUpgrade)
		})
	}
}
