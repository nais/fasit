package upgrader

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
)

func TestIsUpgradeStuck(t *testing.T) {
	upgrader := &ClusterUpgrader{}

	tests := []struct {
		name            string
		clusterUpgrade  *model.ClusterUpgradeStatus
		expectedStuck   bool
	}{
		{
			name: "upgrade stuck for more than 24 hours",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusMasterUpgrade,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-25 * time.Hour),
				StartTime:     time.Now().Add(-25 * time.Hour),
			},
			expectedStuck: true,
		},
		{
			name: "upgrade not stuck, less than 24 hours",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusMasterUpgrade,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-12 * time.Hour),
				StartTime:     time.Now().Add(-12 * time.Hour),
			},
			expectedStuck: false,
		},
		{
			name: "upgrade done, not considered stuck",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusDone,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-30 * time.Hour),
				StartTime:     time.Now().Add(-30 * time.Hour),
			},
			expectedStuck: false,
		},
		{
			name: "upgrade failed, not considered stuck",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusFailed,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-30 * time.Hour),
				StartTime:     time.Now().Add(-30 * time.Hour),
			},
			expectedStuck: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isStuck := upgrader.isUpgradeStuck(tt.clusterUpgrade)
			if isStuck != tt.expectedStuck {
				t.Errorf("isUpgradeStuck() = %v, want %v", isStuck, tt.expectedStuck)
			}
		})
	}
}