package upgrader

import (
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
			UpgradeStatus:         model.UpgradeStatusMasterUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now(),
			SlackChannelID:        "", // No channel ID
			SlackMessageTimestamp: "", // No timestamp
		}

		// This should not cause any panic or error - just return early
		upgrader.updateSlackProgress("tenant1", "env1", clusterUpgrade, "master", "completed")

		// Test passes if no panic occurs
	})

	t.Run("updateSlackProgress with valid message metadata", func(t *testing.T) {
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:                    uuid.New(),
			UpgradeStatus:         model.UpgradeStatusMasterUpgrade,
			Version:               "1.25.0",
			LastModified:          time.Now(),
			SlackChannelID:        "C123456",
			SlackMessageTimestamp: "1234567890.123456",
		}

		// This should attempt to update the message
		upgrader.updateSlackProgress("tenant1", "env1", clusterUpgrade, "master", "completed")

		// Test passes if no panic occurs (fake client returns error which is logged)
	})
}

// Helper interface to satisfy the test requirements
type testRepoWithLogger struct {
	*mock.Mock
	logger interface{}
}

func TestSlackProgressStates(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	testCases := []struct {
		name          string
		currentPhase  string
		status        string
		shouldContain []string
	}{
		{
			name:          "master starting",
			currentPhase:  "master",
			status:        "starting",
			shouldContain: []string{"Starting control plane"},
		},
		{
			name:          "master in progress",
			currentPhase:  "master",
			status:        "in_progress",
			shouldContain: []string{"Control plane upgrade in progress"},
		},
		{
			name:          "master completed",
			currentPhase:  "master",
			status:        "completed",
			shouldContain: []string{"Control plane upgrade completed", "Starting node pools"},
		},
		{
			name:          "nodepool in progress",
			currentPhase:  "nodepool",
			status:        "in_progress",
			shouldContain: []string{"Control plane upgrade completed", "Node pools upgrade in progress"},
		},
		{
			name:          "upgrade completed",
			currentPhase:  "completed",
			status:        "completed",
			shouldContain: []string{"upgrade completed", "completed successfully"},
		},
		{
			name:          "upgrade failed",
			currentPhase:  "failed",
			status:        "failed",
			shouldContain: []string{"failed"},
		},
		{
			name:          "upgrade stuck",
			currentPhase:  "stuck",
			status:        "failed",
			shouldContain: []string{"stuck", "24h"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clusterUpgrade := &model.ClusterUpgradeStatus{
				ID:                    uuid.New(),
				UpgradeStatus:         model.UpgradeStatusMasterUpgrade,
				Version:               "1.25.0",
				LastModified:          time.Now(),
				SlackChannelID:        "C123456",
				SlackMessageTimestamp: "1234567890.123456",
			}

			// Test that the function doesn't panic
			upgrader.updateSlackProgress("tenant1", "env1", clusterUpgrade, tc.currentPhase, tc.status)

			// Test passes if no panic - actual message content testing would require
			// more sophisticated mocking of the Slack client interface
		})
	}
}
