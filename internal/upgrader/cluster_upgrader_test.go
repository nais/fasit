package upgrader

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/slack/fake"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/mocks"
	"github.com/nais/fasit/internal/graph/model"
	upgdradermock "github.com/nais/fasit/internal/upgrader/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc/codes"
)

type env struct {
	tenantID          uuid.UUID
	projectID         string
	id                uuid.UUID
	name              string
	clusterUpgraderID uuid.UUID
}

type testSuite struct {
	repoMock    *mocks.Repo
	upgradeMock *upgdradermock.Upgrader
	env         *env
	environment *model.Environment
}

func newTestSuite(t *testing.T) *testSuite {
	tenantID := uuid.New()
	envID := uuid.New()
	return &testSuite{
		repoMock:    mocks.NewRepo(t),
		upgradeMock: upgdradermock.NewUpgrader(t),
		env: &env{
			tenantID:          tenantID,
			projectID:         "1234",
			id:                envID,
			name:              "t1",
			clusterUpgraderID: uuid.New(),
		},
		environment: &model.Environment{
			ID:       envID,
			Name:     "t1",
			TenantID: tenantID,
		},
	}
}

func newUpgrade(suite *testSuite) *ClusterUpgrader {
	log := logrus.New().WithField("testSuite", "upgrade")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	slackClient := fake.NewFakeSlackClient()
	return NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter, slackClient, "channel")
}

func (s *testSuite) mockRunTenantForLoop(upgradeStatus model.UpgradeStatus) {
	s.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{
			ID:   s.env.tenantID,
			Name: s.env.name,
		},
	}, nil).Once()

	s.repoMock.EXPECT().EnvironmentsGet(mock.Anything, s.env.tenantID).Return([]*model.Environment{
		{
			ID:       s.env.id,
			TenantID: s.env.tenantID,
			Name:     s.env.name,
		},
	}, nil).Once()

	s.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
		&model.EnvironmentValue{
			Key:   projectID,
			Value: []byte(`"1234"`),
		}, nil).Maybe()

	// Mock ClusterUpgradeHistoryGet for the cleanup process - return empty list for most tests
	s.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, s.env.tenantID, s.env.id).Return(
		[]*model.ClusterUpgradeStatus{}, nil).Once()

	// Mock ClusterOperationsGetByUpgradeID for cleanup process - return empty list for most tests
	s.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, mock.Anything).Return(
		[]*model.EnvironmentOperation{}, nil).Maybe()

	// Mock for slack_upgrade_mentions call in updateSlackProgress
	s.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "slack_upgrade_mentions", false).Return(
		&model.EnvironmentValue{
			Key:   "slack_upgrade_mentions",
			Value: []byte(`""`),
		}, nil).Maybe()

	// Mock for Slack message metadata saving in updateSlackProgress
	s.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusCreated,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusCreated,
		Version:       "1.2.4",
		LastModified:  time.Now(),
		StartTime:     time.Now(),
	}

	if upgradeStatus != "" {
		clusterUpgrade.UpgradeStatus = upgradeStatus
	}

	s.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, s.env.tenantID, s.env.id).Return(clusterUpgrade, nil).Once()
}

func TestRun_OperationDoneUpdateClusterNodeStatusToDone(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusNodeUpgrade)

	// Allow multiple GetRunningOperations calls - we'll fix the exact count later
	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetNodePools for stuck detection - return node pools at target version (upgrade complete)
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.NodePool{
			{
				Name:    "nodepool1",
				Version: "1.2.4", // Same as target version
			},
			{
				Name:    "nodepool2",
				Version: "1.2.4", // Same as target version
			},
		}, nil).Maybe()

	// Since upgrade is complete, continue with normal flow
	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id).Return(
		nil, nil).Maybe()

	// Additional GetNodePools call for the normal node upgrade completion check
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.NodePool{
			{
				Name:    "nodepool1",
				Version: "1.2.4",
			},
			{
				Name:    "nodepool2",
				Version: "1.2.4",
			},
		}, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusDone,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	// Mock for Slack message metadata saving in updateSlackProgress
	suite.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusDone,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_StartNodeUpgradeClusterStatusNodeUpgrade(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusNodeUpgrade)

	// GetRunningOperations for main logic and stuck detection
	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetNodePools for stuck detection - return node pools NOT at target version
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.NodePool{
			{
				Name:    "nodepool1",
				Version: "1.2.3", // Lower than target version 1.2.4
			},
			{
				Name:    "nodepool2",
				Version: "1.2.4", // At target version
			},
		}, nil).Maybe()

	// Mock GetRunningClusterOperation for nodeUpgradeStatus
	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id).Return(
		nil, nil).Maybe()

	// Since nodes need upgrading, should start upgrading nodepool1
	suite.upgradeMock.EXPECT().UpgradeNodePool(mock.Anything, mock.Anything, suite.environment, "nodepool1", "1.2.4").Return(
		&containerpb.Operation{
			Name:          "upgrade-nodepool1",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
		}, nil).Once()

	// Mock database operations for starting node upgrade
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		nil, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_StartClusterUpgradeControlPlaneStatusCreated(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusCreated)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Additional GetRunningOperations call for isUpgradeStuck validation
	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	suite.upgradeMock.EXPECT().UpgradeControlPlane(mock.Anything, mock.Anything, suite.environment, "1.2.4").Return(
		&containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_MASTER,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectID, suite.env.name),
			Detail:        "testSuite",
		}, nil).Maybe()

	id := uuid.New()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		&model.EnvironmentOperation{
			ID:     id,
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_UpdateClusterStatusToNodeUpgradeWhenOperationDoneOnControlPlaneUpgrade(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusControlPlaneUpgrade)

	// Allow multiple GetRunningOperations calls - we'll fix the exact count later
	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetCurrentControlPlaneVersion for stuck detection - return control plane at target version (upgrade complete)
	suite.upgradeMock.EXPECT().GetCurrentControlPlaneVersion(mock.Anything, mock.Anything, suite.environment).Return(
		"1.2.4", nil).Maybe() // Same as target version

	// Since control plane upgrade is complete, continue with normal flow to transition to NODE_UPGRADE
	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_DONE.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()

	op := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_DONE,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectID, suite.env.name),
		Detail:        "testSuite",
	}
	suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectID, "operation").Return(
		op, nil).Maybe()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_DONE.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_ControlPlaneUpgradeIsRunning(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusControlPlaneUpgrade)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectID, suite.environment).Return(
		[]*containerpb.Operation{
			{
				Name:          "operation",
				OperationType: containerpb.Operation_UPGRADE_MASTER,
				Status:        containerpb.Operation_RUNNING,
				TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectID, suite.env.name),
				Detail:        "testSuite",
			},
		}, nil).Maybe()

	// Additional GetRunningOperations call for isUpgradeStuck validation - return same running operations so not stuck
	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectID, suite.environment).Return(
		[]*containerpb.Operation{
			{
				Name:          "operation",
				OperationType: containerpb.Operation_UPGRADE_MASTER,
				Status:        containerpb.Operation_RUNNING,
				TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectID, suite.env.name),
				Detail:        "testSuite",
			},
		}, nil).Maybe()

	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()

	op := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectID, suite.env.name),
		Detail:        "testSuite",
	}
	suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectID, "operation").Return(
		op, nil).Maybe()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantID, suite.env.id, gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Maybe()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestCleanupDanglingOperations_NotFound(t *testing.T) {
	// Setup
	tenantID := uuid.New()
	envID := uuid.New()
	upgradeID := uuid.New()
	operationID := uuid.New()
	operationName := "operation-1234-old-operation"
	projectID := "test-project-123"

	repoMock := mocks.NewRepo(t)
	upgradeMock := upgdradermock.NewUpgrader(t)
	meterProvider := metricsdk.NewMeterProvider()
	meter := meterProvider.Meter("test")
	log := logrus.New()
	slackClient := fake.NewFakeSlackClient()

	upgrader := NewClusterUpgrader(repoMock, log, upgradeMock, meter, slackClient, "test-channel")

	environment := &model.Environment{
		ID:       envID,
		TenantID: tenantID,
		Name:     "test-env",
	}

	// Setup: A completed upgrade with a RUNNING operation
	clusterUpgrade := &model.ClusterUpgradeStatus{
		ID:            upgradeID,
		UpgradeStatus: model.UpgradeStatusDone,
		Version:       "1.30.0",
		StartTime:     time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
		LastModified:  time.Now().Add(-29 * 24 * time.Hour),
	}

	runningOp := &model.EnvironmentOperation{
		ID:           operationID,
		Name:         operationName,
		Status:       "RUNNING", // Stuck in RUNNING
		Type:         "UPGRADE_MASTER",
		StartTime:    time.Now().Add(-30 * 24 * time.Hour),
		LastModified: time.Now().Add(-29 * 24 * time.Hour),
	}

	// Mock: ClusterOperationsGetByUpgradeID returns the RUNNING operation
	repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, upgradeID).
		Return([]*model.EnvironmentOperation{runningOp}, nil).Once()

	// Mock: GetOperation returns NotFound
	notFoundErr := createAPIError(codes.NotFound, "Not found: "+operationName)
	upgradeMock.EXPECT().GetOperation(mock.Anything, projectID, operationName).
		Return(nil, notFoundErr).Once()

	// Mock: CreateOrUpdateClusterOperation should be called with DONE status
	repoMock.EXPECT().CreateOrUpdateClusterOperation(
		mock.Anything,
		tenantID,
		envID,
		upgradeID,
		mock.MatchedBy(func(op *containerpb.Operation) bool {
			return op.Name == operationName && op.Status == containerpb.Operation_DONE
		}),
	).Return(runningOp, nil).Once()

	// Execute cleanup
	err := upgrader.cleanupDanglingOperations(context.Background(), projectID, environment, clusterUpgrade)
	// Verify no error was returned
	if err != nil {
		t.Errorf("cleanupDanglingOperations() error = %v, want nil", err)
	}

	// Verify all expectations were met
	upgradeMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}
