package upgrader

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

// MockUpgrader for testing
type MockUpgrader struct {
	mock.Mock
}

func (m *MockUpgrader) GetReleaseChannel(ctx context.Context, projectID string, environment *model.Environment) (string, error) {
	args := m.Called(ctx, projectID, environment)
	return args.String(0), args.Error(1)
}

func (m *MockUpgrader) GetCurrentMasterVersion(ctx context.Context, projectID string, environment *model.Environment) (string, error) {
	args := m.Called(ctx, projectID, environment)
	return args.String(0), args.Error(1)
}

func (m *MockUpgrader) GetAvailableVersions(ctx context.Context, projectID string, environment *model.Environment, releaseChannel string) ([]string, error) {
	args := m.Called(ctx, projectID, environment, releaseChannel)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUpgrader) GetRunningOperations(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.Operation, error) {
	args := m.Called(ctx, projectID, environment)
	return args.Get(0).([]*containerpb.Operation), args.Error(1)
}

func (m *MockUpgrader) UpgradeMaster(ctx context.Context, projectID string, environment *model.Environment, version string) (*containerpb.Operation, error) {
	args := m.Called(ctx, projectID, environment, version)
	return args.Get(0).(*containerpb.Operation), args.Error(1)
}

func (m *MockUpgrader) UpgradeNodePool(ctx context.Context, projectID string, environment *model.Environment, nodePoolName, version string) (*containerpb.Operation, error) {
	args := m.Called(ctx, projectID, environment, nodePoolName, version)
	return args.Get(0).(*containerpb.Operation), args.Error(1)
}

func (m *MockUpgrader) GetOperation(ctx context.Context, projectID, operationID string) (*containerpb.Operation, error) {
	args := m.Called(ctx, projectID, operationID)
	return args.Get(0).(*containerpb.Operation), args.Error(1)
}

func (m *MockUpgrader) GetNodePools(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.NodePool, error) {
	args := m.Called(ctx, projectID, environment)
	return args.Get(0).([]*containerpb.NodePool), args.Error(1)
}

func (m *MockUpgrader) IsTimeInRange(start, end int) bool {
	args := m.Called(start, end)
	return args.Bool(0)
}

func TestIsUpgradeStuck(t *testing.T) {
	ctx := context.Background()
	mockUpgrader := &MockUpgrader{}

	upgrader := &ClusterUpgrader{
		log:    logrus.New(),
		client: mockUpgrader,
	}

	projectID := "test-project"
	env := &model.Environment{
		ID:   uuid.New(),
		Name: "test-env",
	}

	tests := []struct {
		name           string
		clusterUpgrade *model.ClusterUpgradeStatus
		mockOperations []*containerpb.Operation
		expectedStuck  bool
	}{
		{
			name: "CREATED status stuck - been waiting >30min",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusCreated,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-35 * time.Minute),
				StartTime:     time.Now().Add(-35 * time.Minute),
			},
			mockOperations: []*containerpb.Operation{}, // No operations
			expectedStuck:  true,
		},
		{
			name: "CREATED status not stuck - less than 30min",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusCreated,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-20 * time.Minute),
				StartTime:     time.Now().Add(-20 * time.Minute),
			},
			mockOperations: []*containerpb.Operation{},
			expectedStuck:  false,
		},
		{
			name: "MASTER_UPGRADE stuck - no running operations in GKE",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-1 * time.Hour), // Time doesn't matter, only GKE state
				StartTime:     time.Now().Add(-1 * time.Hour),
			},
			mockOperations: []*containerpb.Operation{}, // No running operations
			expectedStuck:  true,
		},
		{
			name: "MASTER_UPGRADE not stuck - has running master upgrade",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-5 * time.Hour), // Even if it's been hours, trust GKE
				StartTime:     time.Now().Add(-5 * time.Hour),
			},
			mockOperations: []*containerpb.Operation{
				{
					Name:          "upgrade-master-op",
					OperationType: containerpb.Operation_UPGRADE_MASTER,
					Status:        containerpb.Operation_RUNNING,
				},
			},
			expectedStuck: false,
		},
		{
			name: "NODE_UPGRADE not stuck - nodes need upgrading, no running operations",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusNodeUpgrade,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-1 * time.Hour), // Time doesn't matter, only GKE state
				StartTime:     time.Now().Add(-1 * time.Hour),
			},
			mockOperations: []*containerpb.Operation{}, // No running operations
			expectedStuck:  false,                      // Not stuck because nodes need upgrading (normal state)
		},
		{
			name: "NODE_UPGRADE not stuck - has running node upgrade",
			clusterUpgrade: &model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusNodeUpgrade,
				Version:       "1.25.0",
				LastModified:  time.Now().Add(-10 * time.Hour), // Even if very long, trust GKE
				StartTime:     time.Now().Add(-10 * time.Hour),
			},
			mockOperations: []*containerpb.Operation{
				{
					Name:          "upgrade-nodes-op",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_RUNNING,
				},
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
			mockOperations: []*containerpb.Operation{},
			expectedStuck:  false,
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
			mockOperations: []*containerpb.Operation{},
			expectedStuck:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock for each test
			mockUpgrader.ExpectedCalls = nil

			// Setup mock expectations - GKE API will always be called except for completed statuses
			switch tt.clusterUpgrade.UpgradeStatus {
			case model.UpgradeStatusDone, model.UpgradeStatusFailed:
				// No API calls needed for completed statuses
			default:
				mockUpgrader.On("GetRunningOperations", ctx, projectID, env).Return(tt.mockOperations, nil).Once()

				// Add completion checking mocks ONLY when no operations are running
				// The completion check is only called when there are no running operations
				if len(tt.mockOperations) == 0 {
					switch tt.clusterUpgrade.UpgradeStatus {
					case model.UpgradeStatusControlPlaneUpgrade:
						// For MASTER_UPGRADE, check if master version matches target
						currentMasterVersion := "1.24.0" // Different version to simulate stuck
						if !tt.expectedStuck {
							// If not expected to be stuck, return target version to simulate completion
							currentMasterVersion = tt.clusterUpgrade.Version
						}
						mockUpgrader.On("GetCurrentMasterVersion", ctx, projectID, env).Return(currentMasterVersion, nil).Once()
					case model.UpgradeStatusNodeUpgrade:
						// For NODE_UPGRADE, check if nodes need upgrading
						nodePoolVersion := "1.24.0" // Lower than target version = needs upgrading = not stuck
						if tt.expectedStuck {
							// If expected to be stuck, all nodes are at target or higher
							nodePoolVersion = tt.clusterUpgrade.Version
						}
						mockNodePools := []*containerpb.NodePool{
							{
								Name:    "default-pool",
								Version: nodePoolVersion,
							},
						}
						mockUpgrader.On("GetNodePools", ctx, projectID, env).Return(mockNodePools, nil).Maybe()
					}
				}
			}

			isStuck := upgrader.isUpgradeStuck(ctx, tt.clusterUpgrade, projectID, env)
			if isStuck != tt.expectedStuck {
				t.Errorf("isUpgradeStuck() = %v, want %v for test: %s", isStuck, tt.expectedStuck, tt.name)
			}

			// Assert all expectations were met
			mockUpgrader.AssertExpectations(t)
		})
	}
}
