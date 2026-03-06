package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func TestClusterNodePoolsCompleted_AllVersionsMatch_NoFailedOps(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - all successful (no nodes_failed)
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !done {
		t.Errorf("expected upgrade to be done when all versions match and no failed operations")
	}
}

func TestClusterNodePoolsCompleted_AllVersionsMatch_HasFailedOps(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version (GKE lies!)
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - some have failed nodes
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "DONE", NodesFailed: 1, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"}, // Failed!
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when operations have failed nodes, even if GKE reports correct version")
	}
}

func TestClusterNodePoolsCompleted_VersionsDontMatch(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - not all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2100000"}, // Old version
	}, nil)

	// Mock operations - not called because version check fails first

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when nodepool versions don't match")
	}
}

func TestClusterNodePoolsCompleted_VersionsBeyondTarget_NoFailedOps(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all above target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.6-gke.2119000"},
		{Name: "pool-2", Version: "1.33.7-gke.2120000"},
	}, nil)

	// Mock operations - all successful
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !done {
		t.Errorf("expected upgrade to be done when all nodepool versions are beyond target and no operations failed")
	}
}

func TestClusterNodePoolsCompleted_MultipleFailedOps(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - multiple failed operations for same pool (retries)
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "DONE", NodesFailed: 1, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 1, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"}, // Retry also failed
		{Name: "op-3", Status: "DONE", NodesFailed: 1, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"}, // Another retry failed
		{Name: "op-4", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when multiple operations have failed nodes")
	}
}

func TestClusterNodePoolsCompleted_FailedThenSuccessfulRetry(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	now := time.Now()
	// Mock operations - pool-1 failed initially but later retry succeeded
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "DONE", NodesFailed: 1, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1", LastModified: now.Add(-10 * time.Minute)}, // Initial failure
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1", LastModified: now},                        // Retry succeeded!
		{Name: "op-3", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2", LastModified: now},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !done {
		t.Errorf("expected upgrade to be done when latest retry succeeded, even with historical failures")
	}
}

func TestClusterNodePoolsCompleted_DatabaseError(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock database error
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return(nil, errors.New("database connection failed"))

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade to NOT be marked done when database call fails (to avoid marking upgrade complete incorrectly)")
	}
}

func TestClusterNodePoolsCompleted_AbortedOperation(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - one operation ABORTED
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "ABORTED", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when operation is ABORTED")
	}
}

func TestClusterNodePoolsCompleted_AbortingOperation(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - one operation ABORTING
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "ABORTING", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when operation is ABORTING")
	}
}

func TestClusterNodePoolsCompleted_OperationStillRunning(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version (GKE reports target version prematurely)
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - one operation still RUNNING
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "RUNNING", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when operation is still RUNNING, even if GKE reports target version")
	}
}

func TestClusterNodePoolsCompleted_OperationStillPending(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := newUpgrade(suite)
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:      uuid.New(),
		Version: "1.33.5-gke.2118000",
	}

	// Mock GetNodePools - all at target version (GKE reports target version prematurely)
	suite.clusterMock.On("GetNodePools", mock.Anything, "test-project", suite.environment).Return([]*containerpb.NodePool{
		{Name: "pool-1", Version: "1.33.5-gke.2118000"},
		{Name: "pool-2", Version: "1.33.5-gke.2118000"},
	}, nil)

	// Mock operations - one operation still PENDING
	suite.repoMock.On("ClusterOperationsGetByUpgradeID", mock.Anything, clusterUpgrade.ID).Return([]*model.EnvironmentOperation{
		{Name: "op-1", Status: "DONE", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-1"},
		{Name: "op-2", Status: "PENDING", NodesFailed: 0, Type: "UPGRADE_NODES", Target: "https://container.googleapis.com/v1/projects/test-project/locations/us-central1/clusters/test-cluster/nodePools/pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when operation is still PENDING, even if GKE reports target version")
	}
}
