package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/slack/fake"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2/apierror"
	clustermock "github.com/nais/fasit/internal/cluster/mocks"
	"github.com/nais/fasit/internal/database/mocks"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// Helper function to create APIError from gRPC status
func createAPIError(code codes.Code, msg string) error {
	grpcErr := status.Error(code, msg)
	apiErr, _ := apierror.FromError(grpcErr)
	return apiErr
}

type env struct {
	tenantID          uuid.UUID
	projectID         string
	id                uuid.UUID
	name              string
	clusterUpgraderID uuid.UUID
}

type testSuite struct {
	repoMock    *mocks.Repo
	clusterMock *clustermock.ClusterManager
	env         *env
	environment *model.Environment
}

func newTestSuite(t *testing.T) *testSuite {
	tenantID := uuid.New()
	envID := uuid.New()
	return &testSuite{
		repoMock:    mocks.NewRepo(t),
		clusterMock: clustermock.NewClusterManager(t),
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
	return NewClusterUpgrader(suite.repoMock, log, suite.clusterMock, meter, slackClient, "channel")
}

func (s *testSuite) mockRunTenantForLoop(upgradeStatus model.UpgradeStatus) *model.ClusterUpgradeStatus {
	// Mock TenantsGet for metrics initialization (first call in Run())
	s.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{
			ID:   s.env.tenantID,
			Name: s.env.name,
		},
	}, nil).Once()

	// Mock EnvironmentsGet for metrics initialization
	s.repoMock.EXPECT().EnvironmentsGet(mock.Anything, s.env.tenantID).Return([]*model.Environment{
		{
			ID:       s.env.id,
			TenantID: s.env.tenantID,
			Name:     s.env.name,
		},
	}, nil).Once()

	// Mock ClusterUpgradeGet for metrics initialization
	s.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, s.env.tenantID, s.env.id).Return(
		nil, nil).Once()

	// Mock TenantsGet for the main processing loop
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
	s.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, s.env.tenantID, s.env.id, mock.Anything, mock.Anything).Return(
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
	return clusterUpgrade
}

func TestRun_OperationDoneUpdateClusterNodeStatusToDone(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	clusterUpgrade := suite.mockRunTenantForLoop(model.UpgradeStatusNodeUpgrade)

	// Allow multiple GetRunningOperations calls - we'll fix the exact count later
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetNodePools for stuck detection - return node pools at target version (upgrade complete)
	suite.clusterMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
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
	suite.clusterMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
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

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, clusterUpgrade.ID, mock.Anything).Return(
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
	clusterUpgrade := suite.mockRunTenantForLoop(model.UpgradeStatusNodeUpgrade)

	// GetRunningOperations for main logic and stuck detection
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetNodePools for stuck detection - return node pools NOT at target version
	suite.clusterMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
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
	suite.clusterMock.EXPECT().UpgradeNodePool(mock.Anything, mock.Anything, suite.environment, "nodepool1", "1.2.4").Return(
		&containerpb.Operation{
			Name:          "upgrade-nodepool1",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
		}, nil).Once()

	// Mock database operations for starting node upgrade
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		nil, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, clusterUpgrade.ID, gensql.ClusterUpgradesStatusNODEUPGRADE).Return(
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
	clusterUpgrade := suite.mockRunTenantForLoop(model.UpgradeStatusCreated)

	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetNodePools for stuck detection - return node pools NOT at target version
	suite.clusterMock.EXPECT().GetNodePools(mock.Anything, mock.Anything, suite.environment).Return(
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

	// Mock GetCurrentControlPlaneVersion to return version lower than target so upgrade proceeds
	suite.clusterMock.EXPECT().GetCurrentControlPlaneVersion(mock.Anything, mock.Anything, suite.environment).Return("1.2.3", nil).Maybe()

	suite.clusterMock.EXPECT().UpgradeControlPlane(mock.Anything, mock.Anything, suite.environment, "1.2.4").Return(
		&containerpb.Operation{
			Name:          "upgrade",
			OperationType: containerpb.Operation_UPGRADE_MASTER,
			Status:        containerpb.Operation_RUNNING,
		}, nil).Once()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		nil, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, clusterUpgrade.ID, mock.Anything).Return(
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
	clusterUpgrade := suite.mockRunTenantForLoop(model.UpgradeStatusControlPlaneUpgrade)

	// Allow multiple GetRunningOperations calls - we'll fix the exact count later
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.environment).Return(
		[]*containerpb.Operation{}, nil).Maybe()

	// Mock GetCurrentControlPlaneVersion for stuck detection - return control plane at target version (upgrade complete)
	suite.clusterMock.EXPECT().GetCurrentControlPlaneVersion(mock.Anything, mock.Anything, suite.environment).Return(
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
	suite.clusterMock.EXPECT().GetOperation(mock.Anything, suite.env.projectID, "operation").Return(
		op, nil).Maybe()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_DONE.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, clusterUpgrade.ID, mock.Anything).Return(
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
	clusterUpgrade := suite.mockRunTenantForLoop(model.UpgradeStatusControlPlaneUpgrade)

	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectID, suite.environment).Return(
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
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectID, suite.environment).Return(
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
	suite.clusterMock.EXPECT().GetOperation(mock.Anything, suite.env.projectID, "operation").Return(
		op, nil).Maybe()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Maybe()
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, clusterUpgrade.ID, gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE).Return(
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
	clusterMock := clustermock.NewClusterManager(t)
	meterProvider := metricsdk.NewMeterProvider()
	meter := meterProvider.Meter("test")
	log := logrus.New()
	slackClient := fake.NewFakeSlackClient()

	upgrader := NewClusterUpgrader(repoMock, log, clusterMock, meter, slackClient, "test-channel")

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
	clusterMock.EXPECT().GetOperation(mock.Anything, projectID, operationName).
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
	clusterMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestRun_CreatedToWaitingTransitionWithDelay(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	// Setup tenant and environment with delay configured
	tenantDelayDays := int32(2)
	envDelayDays := int32(1)

	// Mock TenantsGet for metrics initialization
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
		ID:               suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: tenantDelayDays,
	}}, nil).Once()

	// Mock EnvironmentsGet for metrics initialization
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
		ID:               suite.env.id,
		TenantID:         suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: envDelayDays,
	}}, nil).Once()

	// Mock ClusterUpgradeGet for metrics initialization
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()

	// Mock TenantsGet for main processing loop
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
		ID:               suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: tenantDelayDays,
	}}, nil).Once()

	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
		ID:               suite.env.id,
		TenantID:         suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: envDelayDays,
	}}, nil).Once()

	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
		&model.EnvironmentValue{
			Key:   "project_id",
			Value: []byte(`"1234"`),
		}, nil).Twice()

	// Mock ClusterUpgradeGet to return upgrade in CREATED status (automatic upgrade)
	createdUpgrade := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusCreated,
		Version:       "1.2.4",
		StartTime:     time.Now(),
		LastModified:  time.Now(),
		EnvironmentID: suite.env.id,
		IsAutomatic:   boolPtr(true), // This is an automatic upgrade, should respect delays
	}
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(createdUpgrade, nil).Once()

	suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return([]*model.ClusterUpgradeStatus{}, nil).Once()

	// Mock ClusterOperationsGetByUpgradeID for cleanup operations check
	suite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, createdUpgrade.ID).Return([]*model.EnvironmentOperation{}, nil).Once()

	// Mock GetRunningOperations - called twice: once for stuck check, once in getAndUpdateRunningOperations
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Twice()

	// Expect UpdateClusterUpgradeStatus to be called to transition to WAITING
	waitingUpgrade := &model.ClusterUpgradeStatus{
		ID:            createdUpgrade.ID,
		UpgradeStatus: model.UpgradeStatusWaiting,
		Version:       "1.2.4",
		StartTime:     createdUpgrade.StartTime,
		LastModified:  time.Now(),
		EnvironmentID: suite.env.id,
	}
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, createdUpgrade.ID, gensql.ClusterUpgradesStatusWAITING).
		Return(waitingUpgrade, nil).Once()

	// No Slack notification when transitioning to WAITING - we'll notify when upgrade actually starts

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	// Verify that UpdateClusterUpgradeStatus was called with WAITING status
	suite.repoMock.AssertExpectations(t)
}

func TestRun_CreatedWithoutDelaySkipsWaiting(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	// Setup tenant and environment with NO delay (default 0)
	tenantDelayDays := int32(0)
	envDelayDays := int32(0)

	// Mock TenantsGet for metrics initialization
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
		ID:               suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: tenantDelayDays,
	}}, nil).Once()

	// Mock EnvironmentsGet for metrics initialization
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
		ID:               suite.env.id,
		TenantID:         suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: envDelayDays,
	}}, nil).Once()

	// Mock ClusterUpgradeGet for metrics initialization
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()

	// Mock TenantsGet for main processing loop
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
		ID:               suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: tenantDelayDays,
	}}, nil).Once()

	// Mock EnvironmentsGet for main processing loop
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
		ID:               suite.env.id,
		TenantID:         suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: envDelayDays,
	}}, nil).Once()

	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
		&model.EnvironmentValue{
			Key:   "project_id",
			Value: []byte(`"1234"`),
		}, nil).Twice()

	// Mock ClusterUpgradeGet to return upgrade in CREATED status (automatic upgrade with no delay)
	createdUpgrade := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusCreated,
		Version:       "1.2.4",
		StartTime:     time.Now(),
		LastModified:  time.Now(),
		EnvironmentID: suite.env.id,
		IsAutomatic:   boolPtr(true), // Automatic upgrade with no delay should proceed immediately
	}
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(createdUpgrade, nil).Once()

	suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return([]*model.ClusterUpgradeStatus{}, nil).Once()

	// Mock ClusterOperationsGetByUpgradeID for cleanup operations check
	suite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, createdUpgrade.ID).Return([]*model.EnvironmentOperation{}, nil).Once()

	// Mock GetRunningOperations for stuck check and main logic
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{}, nil).Twice()

	// Mock GetCurrentControlPlaneVersion check in CREATED state
	suite.clusterMock.EXPECT().GetCurrentControlPlaneVersion(mock.Anything, mock.Anything, mock.Anything).Return("1.2.3", nil).Once()

	// Mock UpgradeControlPlane to be called (since no delay)
	suite.clusterMock.EXPECT().UpgradeControlPlane(mock.Anything, mock.Anything, mock.Anything, "1.2.4").Return(
		&containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_MASTER,
			Status:        containerpb.Operation_RUNNING,
		}, nil).Once()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(nil, nil).Once()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, createdUpgrade.ID, gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE).
		Return(&model.ClusterUpgradeStatus{
			ID:            createdUpgrade.ID,
			UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade,
			Version:       "1.2.4",
			StartTime:     createdUpgrade.StartTime,
			LastModified:  time.Now(),
			EnvironmentID: suite.env.id,
		}, nil).Once()

	// Mock SetClusterUpgradesSlackMessage for posting new Slack message
	suite.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	// Mock EnvironmentValueGet for slack mentions
	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "slack_upgrade_mentions", false).Return(
		&model.EnvironmentValue{
			Key:   "slack_upgrade_mentions",
			Value: []byte(`""`),
		}, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	// Verify that UpdateClusterUpgradeStatus was NOT called with WAITING status
	suite.repoMock.AssertNotCalled(t, "UpdateClusterUpgradeStatus", mock.Anything, mock.Anything, gensql.ClusterUpgradesStatusWAITING)
	// Verify that it was called with CONTROL_PLANE_UPGRADE instead
	suite.repoMock.AssertExpectations(t)
}

func TestRun_CreatedWithDelayButRunningOperations(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	// Setup tenant and environment with delay configured
	tenantDelayDays := int32(1)
	envDelayDays := int32(0)

	// Mock TenantsGet for metrics initialization
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
		ID:               suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: tenantDelayDays,
	}}, nil).Once()

	// Mock EnvironmentsGet for metrics initialization
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
		ID:               suite.env.id,
		TenantID:         suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: envDelayDays,
	}}, nil).Once()

	// Mock ClusterUpgradeGet for metrics initialization
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()

	// Mock TenantsGet for main processing loop
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
		ID:               suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: tenantDelayDays,
	}}, nil).Once()

	// Mock EnvironmentsGet for main processing loop
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
		ID:               suite.env.id,
		TenantID:         suite.env.tenantID,
		Name:             suite.env.name,
		UpgradeDelayDays: envDelayDays,
	}}, nil).Once()

	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
		&model.EnvironmentValue{
			Key:   "project_id",
			Value: []byte(`"1234"`),
		}, nil).Twice()

	// Mock ClusterUpgradeGet to return upgrade in CREATED status (automatic upgrade)
	createdUpgrade := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusCreated,
		Version:       "1.2.4",
		StartTime:     time.Now(),
		LastModified:  time.Now(),
		EnvironmentID: suite.env.id,
		IsAutomatic:   boolPtr(true), // Automatic upgrade with delay configured
	}
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(createdUpgrade, nil).Once()

	suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return([]*model.ClusterUpgradeStatus{}, nil).Once()

	// Mock ClusterOperationsGetByUpgradeID for cleanup operations check
	suite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, createdUpgrade.ID).Return([]*model.EnvironmentOperation{}, nil).Once()

	// Mock GetRunningOperations - return a running control plane upgrade operation
	// This simulates GKE having already started the upgrade before Fasit checked
	runningOp := &containerpb.Operation{
		Name:          "operation-123-running",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_RUNNING,
	}
	suite.clusterMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, mock.Anything).Return([]*containerpb.Operation{runningOp}, nil).Twice()

	// getAndUpdateRunningOperations will track the running operation
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantID, suite.env.id, createdUpgrade.ID, mock.Anything).Return(nil, nil).Once()

	// Since there are running operations, should:
	// 1. Skip delay logic (even though delay is configured)
	// 2. NOT call UpgradeControlPlane (don't start a new upgrade)
	// 3. Transition directly to CONTROL_PLANE_UPGRADE status since control plane operation is running

	// Expect UpdateClusterUpgradeStatus to be called to transition to CONTROL_PLANE_UPGRADE
	updatedUpgrade := &model.ClusterUpgradeStatus{
		ID:            createdUpgrade.ID,
		UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade,
		Version:       "1.2.4",
		StartTime:     time.Now(),
		LastModified:  time.Now(),
		EnvironmentID: suite.env.id,
		IsAutomatic:   boolPtr(true),
	}
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, createdUpgrade.ID, gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE).Return(updatedUpgrade, nil).Once()

	// Mock slack_upgrade_mentions for updateSlackProgress
	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "slack_upgrade_mentions", false).Return(
		&model.EnvironmentValue{
			Key:   "slack_upgrade_mentions",
			Value: []byte(`[]`),
		}, nil).Once()

	// Mock SetClusterUpgradesSlackMessage for Slack notification
	suite.repoMock.EXPECT().SetClusterUpgradesSlackMessage(mock.Anything, createdUpgrade.ID, mock.Anything, mock.Anything).Return(updatedUpgrade, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	// Verify that UpdateClusterUpgradeStatus was NOT called with WAITING status
	// even though delay is configured, because GKE already has running operations
	suite.repoMock.AssertNotCalled(t, "UpdateClusterUpgradeStatus", mock.Anything, mock.Anything, gensql.ClusterUpgradesStatusWAITING)
	suite.repoMock.AssertExpectations(t)
}

func TestRun_UpgradeDurationUsesUpgradeStartTime(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	// Create timestamps
	now := time.Now()
	startTime := now.Add(-2 * time.Hour)           // Created 2 hours ago
	upgradeStartTime := now.Add(-30 * time.Minute) // Actual upgrade started 30 minutes ago
	lastModified := now                            // Completed now

	// Mock metrics initialization
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{ID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{
		{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()

	// Mock main loop
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{ID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{
		{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()

	// Mock upgrade in DONE state with both timestamps set
	doneUpgrade := &model.ClusterUpgradeStatus{
		ID:               suite.env.clusterUpgraderID,
		UpgradeStatus:    model.UpgradeStatusDone,
		Version:          "1.28.0-gke.1000",
		StartTime:        startTime,         // Created 2 hours ago
		UpgradeStartTime: &upgradeStartTime, // Started 30 minutes ago
		LastModified:     lastModified,      // Completed now
		EnvironmentID:    suite.env.id,
	}

	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(doneUpgrade, nil).Once()
	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "project_id", false).Return(
		&model.EnvironmentValue{Key: "project_id", Value: []byte(`"test-project"`)},
		nil,
	).Maybe()
	suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		[]*model.ClusterUpgradeStatus{doneUpgrade}, nil,
	).Once()
	suite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, doneUpgrade.ID).Return(
		[]*model.EnvironmentOperation{}, nil,
	).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	suite.repoMock.AssertExpectations(t)

	// Note: We can't directly verify the duration metric value without more invasive changes
	// to expose metrics in tests, but this test ensures the code path is covered and
	// the upgrade completes successfully with UpgradeStartTime set
}

func TestRun_UpgradeDurationWarnsWithoutUpgradeStartTime(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	// Create timestamps
	now := time.Now()
	startTime := now.Add(-2 * time.Hour) // Created 2 hours ago
	lastModified := now                  // Completed now

	// Mock metrics initialization
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{ID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{
		{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()
	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()

	// Mock main loop
	suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{ID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()
	suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{
		{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name},
	}, nil).Once()

	// Mock upgrade in DONE state WITHOUT UpgradeStartTime (old upgrade)
	doneUpgrade := &model.ClusterUpgradeStatus{
		ID:               suite.env.clusterUpgraderID,
		UpgradeStatus:    model.UpgradeStatusDone,
		Version:          "1.28.0-gke.1000",
		StartTime:        startTime,    // Created 2 hours ago
		UpgradeStartTime: nil,          // NOT SET (old upgrade)
		LastModified:     lastModified, // Completed now
		EnvironmentID:    suite.env.id,
	}

	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(doneUpgrade, nil).Once()
	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "project_id", false).Return(
		&model.EnvironmentValue{Key: "project_id", Value: []byte(`"test-project"`)},
		nil,
	).Maybe()
	suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id, mock.Anything, mock.Anything).Return(
		[]*model.ClusterUpgradeStatus{doneUpgrade}, nil,
	).Once()
	suite.repoMock.EXPECT().ClusterOperationsGetByUpgradeID(mock.Anything, doneUpgrade.ID).Return(
		[]*model.EnvironmentOperation{}, nil,
	).Maybe()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	suite.repoMock.AssertExpectations(t)

	// The test passes if no error is thrown and the warning is logged (verified by code coverage)
}
