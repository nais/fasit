package upgrader

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nais/fasit/pkg/database/gensql"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/graph/model"
	upgdradermock "github.com/nais/fasit/pkg/upgrader/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

type env struct {
	tenantId          uuid.UUID
	projectId         string
	id                uuid.UUID
	name              string
	clusterUpgraderId uuid.UUID
}

type testSuite struct {
	repoMock    *mocks.Repo
	upgradeMock *upgdradermock.Upgrader
	env         *env
}

func newTestSuite(t *testing.T) *testSuite {
	return &testSuite{
		repoMock:    mocks.NewRepo(t),
		upgradeMock: upgdradermock.NewUpgrader(t),
		env: &env{
			tenantId:          uuid.New(),
			projectId:         "1234",
			id:                uuid.New(),
			name:              "t1",
			clusterUpgraderId: uuid.New(),
		},
	}
}

func newUpgrade(suite *testSuite) *ClusterUpgrader {
	log := logrus.New().WithField("testSuite", "upgrade")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	return NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)
}

func (s *testSuite) mockRunTenantForLoop(upgradeStatus model.UpgradeStatus) {
	s.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{
		{
			ID:   s.env.tenantId,
			Name: s.env.name,
		},
	}, nil).Once()

	s.repoMock.EXPECT().EnvironmentsGet(mock.Anything, s.env.tenantId).Return([]*model.Environment{
		{
			ID:       s.env.id,
			TenantID: s.env.tenantId,
			Name:     s.env.name,
		},
	}, nil).Once()

	s.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, mock.Anything, false).Return(
		&model.EnvironmentValue{
			Key:   projectId,
			Value: []byte(`"1234"`),
		}, nil).Once()

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

	s.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, s.env.tenantId, s.env.id).Return(clusterUpgrade, nil).Once()
}

func TestRun_OperationDoneUpdateClusterNodeStatusToDone(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusNodeUpgrade)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectId, suite.env.name).Return(
		[]*containerpb.Operation{}, nil).Once()
	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).Return(
		nil, nil).Once()

	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).Return(
		[]*containerpb.NodePool{
			{
				Name:    "nodepool1",
				Version: "1.2.4",
			},
			{
				Name:    "nodepool2",
				Version: "1.2.4",
			},
		}, nil).Once()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusDONE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusDone,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_StartNodeUpgradeClusterStatusNodeUpgrade(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusNodeUpgrade)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectId, suite.env.name).Return(
		[]*containerpb.Operation{}, nil).Once()
	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_NODES",
		}, nil).Once()

	op := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_NODES,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}

	suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, "operation").Return(
		op, nil).Once()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_NODES",
		}, nil).Once()

	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).Return(
		[]*containerpb.NodePool{
			{
				Name:    "nodepool1",
				Version: "1.2.3",
			},
			{
				Name:    "nodepool2",
				Version: "1.2.4",
			},
		}, nil).Twice()

	op2 := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_NODES,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}

	suite.upgradeMock.EXPECT().UpgradeNodePool(mock.Anything, suite.env.projectId, suite.env.name, "nodepool1", "1.2.4").Return(
		op2, nil).Once()
	id := uuid.New()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, op2).Return(
		&model.EnvironmentOperation{
			ID:     id,
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_NODES",
		}, nil).Once()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_StartClusterUpgradeMasterStatusCreated(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusCreated)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.env.name).Return(
		[]*containerpb.Operation{}, nil).Twice()

	suite.upgradeMock.EXPECT().UpgradeMaster(mock.Anything, mock.Anything, suite.env.name, "1.2.4").Return(
		&containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_MASTER,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
			Detail:        "testSuite",
		}, nil).Once()

	id := uuid.New()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, mock.Anything).Return(
		&model.EnvironmentOperation{
			ID:     id,
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Once()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusMASTERUPGRADE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_UpdateClusterStatusToNodeUpgradeWhenOperationDoneOnMasterUpgrade(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusMasterUpgrade)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.env.name).Return(
		[]*containerpb.Operation{}, nil).Once()

	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_DONE.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Once()

	op := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_DONE,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}
	suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, "operation").Return(
		op, nil).Once()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_DONE.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Once()

	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestRun_MasterUpgradeIsRunning(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)
	suite.mockRunTenantForLoop(model.UpgradeStatusMasterUpgrade)

	suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, suite.env.projectId, suite.env.name).Return(
		[]*containerpb.Operation{
			{
				Name:          "operation",
				OperationType: containerpb.Operation_UPGRADE_MASTER,
				Status:        containerpb.Operation_RUNNING,
				TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
				Detail:        "testSuite",
			},
		}, nil).Once()
	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Once()

	op := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}
	suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, "operation").Return(
		op, nil).Once()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, op).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Once()
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusMASTERUPGRADE, "1.2.4").Return(
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}, nil).Once()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, mock.Anything).Return(
		&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}, nil).Once()

	err := upgrade.Run(context.Background())
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func Test_EqualVersionsForAllNodes(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	nodepools := []*containerpb.NodePool{
		{
			Name:    "nodepool1",
			Version: "1.2.3",
		},
		{
			Name:    "nodepool2",
			Version: "1.2.4",
		},
	}
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
		Return(nodepools, nil).Once()
	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusNodeUpgrade,
		Version:       "1.2.4",
		LastModified:  time.Now(),
		StartTime:     time.Now(),
	}
	operation := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_NODES,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}
	// We hit the one with the nodepool1 version 1.2.3
	suite.upgradeMock.EXPECT().UpgradeNodePool(mock.Anything, suite.env.projectId, suite.env.name, nodepools[0].Name, clusterUpgradeStatus.Version).
		Return(operation, nil).Once()

	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
		Return(&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_NODES",
		}, nil).Once()

	clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusNodeUpgrade
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgradeStatus.Version).
		Return(clusterUpgradeStatus, nil).Once()

	cu, err := upgrade.upgradeNodes(context.Background(), &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Created:      time.Now(),
		LastModified: time.Now(),
	}, clusterUpgradeStatus, suite.env.projectId, suite.env.name)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	if clusterUpgradeStatus.Version != cu.Version {
		t.Errorf("got %v, want %v", clusterUpgradeStatus.Version, cu.Version)
	}
	if model.UpgradeStatusNodeUpgrade != cu.UpgradeStatus {
		t.Errorf("got %v, want %v", clusterUpgradeStatus.UpgradeStatus, cu.UpgradeStatus)
	}
}

func Test_NodelPoolDiff(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	nodepools := []*containerpb.NodePool{
		{
			Name:    "nodepool1",
			Version: "1.2.4",
		},
		{
			Name:    "nodepool2",
			Version: "1.2.4",
		},
	}

	suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).
		Return(&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_NODES",
		}, nil).Once()

	operation := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_NODES,
		Status:        containerpb.Operation_DONE,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}

	suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, operation.Name).
		Return(operation, nil).Once()

	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusNodeUpgrade,
		Version:       "1.2.4",
		LastModified:  time.Now(),
		StartTime:     time.Now(),
	}
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
		Return(&model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_NODES",
		}, nil).Once()
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
		Return(nodepools, nil).Once()

	done, err := upgrade.nodeUpgradeStatus(context.Background(), &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	}, clusterUpgradeStatus, suite.env.projectId)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if !done {
		t.Errorf("got %v, want true", done)
	}
}

func Test_ClusterNodePoolsCompleted(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	nodepools := []*containerpb.NodePool{
		{
			Name:    "nodepool1",
			Version: "1.2.4",
		},
		{
			Name:    "nodepool2",
			Version: "1.2.4",
		},
	}
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
		Return(nodepools, nil).Once()

	completed, err := upgrade.clusterNodePoolsCompleted(context.Background(), suite.env.projectId, &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	},
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		})
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if !completed {
		t.Errorf("got %v, want true", completed)
	}

	nodepools = []*containerpb.NodePool{
		{
			Name:    "nodepool1",
			Version: "1.2.3",
		},
		{
			Name:    "nodepool2",
			Version: "1.2.4",
		},
	}
	suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
		Return(nodepools, nil).Once()

	completed, err = upgrade.clusterNodePoolsCompleted(context.Background(), suite.env.projectId, &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	},
		&model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusNodeUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		})
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if completed {
		t.Errorf("got %v, want false", completed)
	}
}

func Test_MasterUpgrade(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusCreated,
		Version:       "1.2.3",
		LastModified:  time.Now(),
		StartTime:     time.Now(),
	}
	operation := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}
	suite.upgradeMock.EXPECT().UpgradeMaster(mock.Anything, suite.env.projectId, suite.env.name, clusterUpgradeStatus.Version).
		Return(operation, nil).Once()
	suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
		Return(nil, nil).Once()

	if clusterUpgradeStatus.UpgradeStatus != model.UpgradeStatusCreated {
		t.Errorf("got %v, want %v", clusterUpgradeStatus.UpgradeStatus, model.UpgradeStatusCreated)
	}

	clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusMasterUpgrade
	suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusMASTERUPGRADE, clusterUpgradeStatus.Version).
		Return(clusterUpgradeStatus, nil).Once()

	cus, err := upgrade.masterUpgrade(context.Background(), &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	}, clusterUpgradeStatus, suite.env.name, suite.env.projectId)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if clusterUpgradeStatus.Version != cus.Version {
		t.Errorf("got %v, want %v", clusterUpgradeStatus.Version, cus.Version)
	}
	if model.UpgradeStatusMasterUpgrade != cus.UpgradeStatus {
		t.Errorf("got %v, want %v", clusterUpgradeStatus.UpgradeStatus, cus.UpgradeStatus)
	}
}

func Test_MasterUpgradeStatusIsDone(t *testing.T) {
	suite := newTestSuite(t)
	upgrade := newUpgrade(suite)

	statuses := []containerpb.Operation_Status{
		containerpb.Operation_DONE,
		containerpb.Operation_RUNNING,
	}

	for _, status := range statuses {
		envOp := &model.EnvironmentOperation{
			ID:     uuid.New(),
			Name:   "operation",
			Status: containerpb.Operation_RUNNING.String(),
			Type:   "UPGRADE_MASTER",
		}

		// Get cluster upgrade status
		suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).
			Return(envOp, nil).Once()
		operation := &containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_MASTER,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
			Detail:        "testSuite",
		}
		suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, operation.Name).
			Return(operation, nil).Once()
		envOp.Status = status.String()
		operation.Status = status
		clusterUpgradeStatus := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.2.3",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}
		suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
			Return(envOp, nil).Once()

		// Master upgrade finished - start node upgrade
		if status == containerpb.Operation_DONE {
			fmt.Println("status is done")
			clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusNodeUpgrade
			suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgradeStatus.Version).
				Return(clusterUpgradeStatus, nil).Once()
		}
		cus, err := upgrade.masterUpgradeStatus(context.Background(), &model.Environment{
			ID:       suite.env.id,
			TenantID: suite.env.tenantId,
			Name:     suite.env.name,
		}, clusterUpgradeStatus, suite.env.projectId, suite.env.name)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if cus != nil {
			if clusterUpgradeStatus.Version != cus.Version {
				t.Errorf("got %v, want %v", clusterUpgradeStatus.Version, cus.Version)
			}
			if model.UpgradeStatusNodeUpgrade != cus.UpgradeStatus {
				t.Errorf("got %v, want %v", clusterUpgradeStatus.UpgradeStatus, cus.UpgradeStatus)
			}
		}
	}
}
