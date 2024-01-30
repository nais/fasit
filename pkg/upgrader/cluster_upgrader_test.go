package upgrader

import (
	"context"
	"fmt"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

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

func Test_Run_Created(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	t.Run("should start a cluster upgrade of MASTER when cluster status is CREATED", func(t *testing.T) {
		suite.repoMock.EXPECT().TenantsGet(ctx).Return([]*model.Tenant{
			{
				ID:   suite.env.tenantId,
				Name: "t1",
			},
		}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentsGet(ctx, suite.env.tenantId).Return([]*model.Environment{
			{
				ID:       suite.env.id,
				TenantID: suite.env.tenantId,
				Name:     suite.env.name,
			},
		}, nil).Once()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, mock.Anything, false).Return(
			&model.EnvironmentValue{
				Key:   projectId,
				Value: []byte(`"1"`),
			}, nil).Once()

		suite.repoMock.EXPECT().ClusterUpgradeGet(ctx, suite.env.tenantId, suite.env.id).Return(
			&model.ClusterUpgradeStatus{
				ID:            uuid.New(),
				UpgradeStatus: model.UpgradeStatusCreated,
				Version:       "1.2.4",
				LastModified:  time.Now(),
				StartTime:     time.Now(),
			}, nil).Once()

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

		err := upgrader.Run(ctx)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
	})
}

func Test_Run_MasterUpgrade_Created(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	suite.mockTenantForLoop(nil)

	t.Run("should update cluster node status to NODE_UPGRADE if Operation DONE and status is MASTER_UPGRADE", func(t *testing.T) {
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.env.name).Return(
			[]*containerpb.Operation{}, nil).Once()

		suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).Return(
			&model.EnvironmentOperation{
				ID:     uuid.New(),
				Name:   "operation",
				Status: containerpb.Operation_DONE.String(),
				Type:   "UPGRADE_MASTER",
			}, nil).Once()

		suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, "operation").Return(
			&containerpb.Operation{
				Name:          "operation",
				OperationType: containerpb.Operation_UPGRADE_MASTER,
				Status:        containerpb.Operation_DONE,
				TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
				Detail:        "testSuite",
			}, nil).Once()

		suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, mock.Anything).Return(
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
	})

	err := upgrader.Run(ctx)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func Test_Run_MasterUpgrade_Running(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	suite.mockTenantForLoop(nil)

	t.Run("should continue loop if cluster upgrade status is MASTER_UPGRADE and operation is RUNNING", func(t *testing.T) {
		suite.upgradeMock.EXPECT().GetRunningOperations(mock.Anything, mock.Anything, suite.env.name).Return(
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
		suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, "operation").Return(
			&containerpb.Operation{
				Name:          "operation",
				OperationType: containerpb.Operation_UPGRADE_MASTER,
				Status:        containerpb.Operation_RUNNING,
				TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
				Detail:        "testSuite",
			}, nil).Once()
		suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, mock.Anything, mock.Anything).Return(
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

		err := upgrader.Run(ctx)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
	})
}

func Test_UpgradeNodes(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	e := &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Created:      time.Now(),
		LastModified: time.Now(),
	}

	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:            uuid.New(),
		UpgradeStatus: model.UpgradeStatusNodeUpgrade,
		Version:       "1.2.4",
		LastModified:  time.Now(),
		StartTime:     time.Now(),
	}

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

	t.Run("should get cluster nodepools and upgrade node if version is not equal", func(t *testing.T) {

		suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
			Return(nodepools, nil).Once()

		operation := &containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
			Detail:        "testSuite",
		}
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

		cu, err := upgrader.upgradeNodes(ctx, e, clusterUpgradeStatus, suite.env.projectId, suite.env.name)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, clusterUpgradeStatus.Version, cu.Version)
		assert.Equal(t, model.UpgradeStatusNodeUpgrade, cu.UpgradeStatus)
		assert.Equal(t, clusterUpgradeStatus.StartTime, cu.StartTime)
		assert.Equal(t, clusterUpgradeStatus.LastModified, cu.LastModified)
		assert.Equal(t, clusterUpgradeStatus.RunningOperations, cu.RunningOperations)
		assert.Equal(t, clusterUpgradeStatus.ID, cu.ID)
	})
}

func Test_NodeUpgradeStatus(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	e := &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	}
	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:                uuid.New(),
		UpgradeStatus:     model.UpgradeStatusNodeUpgrade,
		Version:           "1.2.4",
		LastModified:      time.Now(),
		StartTime:         time.Now(),
		RunningOperations: nil,
	}

	envOp := &model.EnvironmentOperation{
		ID:     uuid.New(),
		Name:   "operation",
		Status: containerpb.Operation_RUNNING.String(),
		Type:   "UPGRADE_NODES",
	}

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

	t.Run("should get operations and update, check nodepools for version diff ", func(t *testing.T) {

		suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).
			Return(envOp, nil).Once()

		operation := &containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_DONE,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
			Detail:        "testSuite",
		}

		suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, operation.Name).
			Return(operation, nil).Once()
		suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
			Return(envOp, nil).Once()
		suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
			Return(nodepools, nil).Once()

		done, err := upgrader.nodeUpgradeStatus(ctx, e, clusterUpgradeStatus, suite.env.projectId, suite.env.name)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, true, done)
	})
}

func Test_ClusterNodePoolsCompleted(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	e := &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	}
	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:                uuid.New(),
		UpgradeStatus:     model.UpgradeStatusNodeUpgrade,
		Version:           "1.2.4",
		LastModified:      time.Now(),
		StartTime:         time.Now(),
		RunningOperations: nil,
	}

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

	t.Run("should validate the state of all nodepools ", func(t *testing.T) {

		suite.upgradeMock.EXPECT().GetNodePools(mock.Anything, suite.env.projectId, suite.env.name).
			Return(nodepools, nil).Once()

		completed, err := upgrader.clusterNodePoolsCompleted(ctx, suite.env.projectId, e, clusterUpgradeStatus)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, completed, true)

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

		completed, err = upgrader.clusterNodePoolsCompleted(ctx, suite.env.projectId, e, clusterUpgradeStatus)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, false, completed)
	})
}

func Test_MasterUpgrade(t *testing.T) {
	ctx := context.Background()
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)

	e := &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	}
	clusterUpgradeStatus := &model.ClusterUpgradeStatus{
		ID:                uuid.New(),
		UpgradeStatus:     model.UpgradeStatusCreated,
		Version:           "1.2.3",
		LastModified:      time.Now(),
		StartTime:         time.Now(),
		RunningOperations: nil,
	}

	operation := &containerpb.Operation{
		Name:          "operation",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_RUNNING,
		TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
		Detail:        "testSuite",
	}

	t.Run("should update master and update cluster upgrade status", func(t *testing.T) {
		suite.upgradeMock.EXPECT().UpgradeMaster(mock.Anything, suite.env.projectId, suite.env.name, clusterUpgradeStatus.Version).
			Return(operation, nil).Once()
		suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
			Return(nil, nil).Once()
		assert.Equal(t, clusterUpgradeStatus.UpgradeStatus, model.UpgradeStatusCreated)

		clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusMasterUpgrade
		suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusMASTERUPGRADE, clusterUpgradeStatus.Version).
			Return(clusterUpgradeStatus, nil).Once()

		cus, err := upgrader.masterUpgrade(ctx, e, clusterUpgradeStatus, suite.env.name, suite.env.projectId)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, clusterUpgradeStatus.Version, cus.Version)
		assert.Equal(t, model.UpgradeStatusMasterUpgrade, cus.UpgradeStatus)
		assert.Equal(t, clusterUpgradeStatus.StartTime, cus.StartTime)
		assert.Equal(t, clusterUpgradeStatus.LastModified, cus.LastModified)
		assert.Equal(t, clusterUpgradeStatus.RunningOperations, cus.RunningOperations)
		assert.Equal(t, clusterUpgradeStatus.ID, cus.ID)
	})
}
func Test_MasterUpgradeStatus(t *testing.T) {
	suite := newTestSuite(t)
	log := logrus.New().WithField("testSuite", "upgrader")
	meter := metricsdk.NewMeterProvider().Meter("testSuite")
	upgrader := NewClusterUpgrader(suite.repoMock, log, suite.upgradeMock, meter)
	e := &model.Environment{
		ID:           suite.env.id,
		TenantID:     suite.env.tenantId,
		Name:         suite.env.name,
		Description:  nil,
		Created:      time.Now(),
		LastModified: time.Now(),
	}

	statuses := []containerpb.Operation_Status{
		containerpb.Operation_DONE,
		containerpb.Operation_RUNNING,
	}

	t.Run("should validate master upgrade status for DONE and label node upgrade", func(t *testing.T) {

		for _, status := range statuses {
			clusterUpgradeStatus := &model.ClusterUpgradeStatus{
				ID:                uuid.New(),
				UpgradeStatus:     model.UpgradeStatusMasterUpgrade,
				Version:           "1.2.3",
				LastModified:      time.Now(),
				StartTime:         time.Now(),
				RunningOperations: nil,
			}

			operation := &containerpb.Operation{
				Name:          "operation",
				OperationType: containerpb.Operation_UPGRADE_MASTER,
				Status:        containerpb.Operation_RUNNING,
				TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
				Detail:        "testSuite",
			}

			envOp := &model.EnvironmentOperation{
				ID:     uuid.New(),
				Name:   "operation",
				Status: containerpb.Operation_RUNNING.String(),
				Type:   "UPGRADE_MASTER",
			}

			// Get cluster upgrade status
			suite.repoMock.EXPECT().GetRunningClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id).
				Return(envOp, nil).Once()
			suite.upgradeMock.EXPECT().GetOperation(mock.Anything, suite.env.projectId, operation.Name).
				Return(operation, nil).Once()
			envOp.Status = status.String()
			operation.Status = status
			suite.repoMock.EXPECT().CreateOrUpdateClusterOperation(mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
				Return(envOp, nil).Once()
			clusterUpgradeStatus.RunningOperations = []*model.EnvironmentOperation{envOp}

			// Master upgrade finished - start node upgrade
			if status == containerpb.Operation_DONE {
				clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusNodeUpgrade
				suite.repoMock.EXPECT().UpdateClusterUpgradeStatus(mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgradeStatus.Version).
					Return(clusterUpgradeStatus, nil).Once()
			}

			cus, err := upgrader.masterUpgradeStatus(context.Background(), e, clusterUpgradeStatus, suite.env.projectId, suite.env.name)
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}

			// Test Master upgrade finished
			if status == containerpb.Operation_DONE {
				if cus == nil {
					t.Errorf("got nil, want operation")
				}
				// Upgrading nodes
				assert.Equal(t, clusterUpgradeStatus.Version, cus.Version)
				assert.Equal(t, model.UpgradeStatusNodeUpgrade, cus.UpgradeStatus)
				assert.Equal(t, clusterUpgradeStatus.StartTime, cus.StartTime)
				assert.Equal(t, clusterUpgradeStatus.LastModified, cus.LastModified)
				assert.Equal(t, clusterUpgradeStatus.RunningOperations, cus.RunningOperations)
				assert.Equal(t, clusterUpgradeStatus.ID, cus.ID)
			}
		}
	})
}

func (s *testSuite) mockTenantForLoop(upgradeStatus *model.ClusterUpgradeStatus) {
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

	if upgradeStatus == nil {
		upgradeStatus = &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusMasterUpgrade,
			Version:       "1.2.4",
			LastModified:  time.Now(),
			StartTime:     time.Now(),
		}
	}
	s.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, s.env.tenantId, s.env.id).Return(upgradeStatus, nil).Once()
}
