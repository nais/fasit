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

func TestUpgradeNodes(t *testing.T) {
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

		suite.upgradeMock.On("GetNodePools", mock.Anything, suite.env.projectId, suite.env.name).
			Return(nodepools, nil).Once()

		operation := &containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
			Detail:        "testSuite",
		}
		suite.upgradeMock.On("UpgradeNodePool", mock.Anything, suite.env.projectId, suite.env.name, nodepools[0].Name, clusterUpgradeStatus.Version).
			Return(operation, nil).Once()

		suite.repoMock.On("CreateOrUpdateClusterOperation", mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
			Return(&model.EnvironmentOperation{
				ID:     uuid.New(),
				Name:   "operation",
				Status: containerpb.Operation_RUNNING.String(),
				Type:   "UPGRADE_NODES",
			}, nil).Once()

		clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusNodeUpgrade
		suite.repoMock.On("UpdateClusterUpgradeStatus", mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgradeStatus.Version).
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

func TestNodeUpgradeStatus(t *testing.T) {
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

		suite.repoMock.On("GetRunningClusterOperation", mock.Anything, suite.env.tenantId, suite.env.id).
			Return(envOp, nil).Once()

		operation := &containerpb.Operation{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_DONE,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", suite.env.projectId, suite.env.name),
			Detail:        "testSuite",
		}

		suite.upgradeMock.On("GetOperation", mock.Anything, suite.env.projectId, operation.Name).
			Return(operation, nil).Once()
		suite.repoMock.On("CreateOrUpdateClusterOperation", mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
			Return(envOp, nil).Once()
		suite.upgradeMock.On("GetNodePools", mock.Anything, suite.env.projectId, suite.env.name).
			Return(nodepools, nil).Once()

		done, err := upgrader.nodeUpgradeStatus(ctx, e, clusterUpgradeStatus, suite.env.projectId, suite.env.name)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, true, done)
	})
}

func TestClusterNodePoolsCompleted(t *testing.T) {
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

		suite.upgradeMock.On("GetNodePools", mock.Anything, suite.env.projectId, suite.env.name).
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
		suite.upgradeMock.On("GetNodePools", mock.Anything, suite.env.projectId, suite.env.name).
			Return(nodepools, nil).Once()

		completed, err = upgrader.clusterNodePoolsCompleted(ctx, suite.env.projectId, e, clusterUpgradeStatus)
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		assert.Equal(t, false, completed)
	})
}

func TestMasterUpgrade(t *testing.T) {
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

		suite.upgradeMock.On("UpgradeMaster", mock.Anything, suite.env.projectId, suite.env.name, clusterUpgradeStatus.Version).
			Return(operation, nil).Once()
		suite.repoMock.On("CreateOrUpdateClusterOperation", mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
			Return(nil, nil).Once()
		assert.Equal(t, clusterUpgradeStatus.UpgradeStatus, model.UpgradeStatusCreated)

		clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusMasterUpgrade
		suite.repoMock.On("UpdateClusterUpgradeStatus", mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusMASTERUPGRADE, clusterUpgradeStatus.Version).
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
func TestMasterUpgradeStatus(t *testing.T) {
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
			suite.repoMock.On("GetRunningClusterOperation", mock.Anything, suite.env.tenantId, suite.env.id).
				Return(envOp, nil).Once()
			suite.upgradeMock.On("GetOperation", mock.Anything, suite.env.projectId, operation.Name).
				Return(operation, nil).Once()
			envOp.Status = status.String()
			operation.Status = status
			suite.repoMock.On("CreateOrUpdateClusterOperation", mock.Anything, suite.env.tenantId, suite.env.id, clusterUpgradeStatus.ID, operation).
				Return(envOp, nil).Once()
			clusterUpgradeStatus.RunningOperations = []*model.EnvironmentOperation{envOp}

			// Master upgrade finished - start node upgrade
			if status == containerpb.Operation_DONE {
				clusterUpgradeStatus.UpgradeStatus = model.UpgradeStatusNodeUpgrade
				suite.repoMock.On("UpdateClusterUpgradeStatus", mock.Anything, suite.env.tenantId, suite.env.id, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgradeStatus.Version).
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
