package cluster

import (
	"context"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

func TestClusterNodePoolsCompleted_AllVersionsMatch_NoFailedOps(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := setupUpgrader(suite)
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
		{Name: "op-1", Status: "DONE", NodesFailed: 0, Target: "pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Target: "pool-2"},
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

	upgrade := setupUpgrader(suite)
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
		{Name: "op-1", Status: "DONE", NodesFailed: 1, Target: "pool-1"}, // Failed!
		{Name: "op-2", Status: "DONE", NodesFailed: 0, Target: "pool-2"},
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

	upgrade := setupUpgrader(suite)
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

func TestClusterNodePoolsCompleted_MultipleFailedOps(t *testing.T) {
	suite := newTestSuite(t)
	ctx := context.Background()

	upgrade := setupUpgrader(suite)
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
		{Name: "op-1", Status: "DONE", NodesFailed: 1, Target: "pool-1"},
		{Name: "op-2", Status: "DONE", NodesFailed: 1, Target: "pool-1"}, // Retry also failed
		{Name: "op-3", Status: "DONE", NodesFailed: 1, Target: "pool-1"}, // Another retry failed
		{Name: "op-4", Status: "DONE", NodesFailed: 0, Target: "pool-2"},
	}, nil)

	done, err := upgrade.clusterNodePoolsCompleted(ctx, "test-project", suite.environment, clusterUpgrade)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if done {
		t.Errorf("expected upgrade NOT done when multiple operations have failed nodes")
	}
}

func setupUpgrader(suite *testSuite) *ClusterUpgrader {
	meter := metricsdk.NewMeterProvider().Meter("test")
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during tests
	slackClient := fake.NewFakeSlackClient()

	return NewClusterUpgrader(suite.repoMock, logger, suite.clusterMock, meter, slackClient, "test-channel")
}
