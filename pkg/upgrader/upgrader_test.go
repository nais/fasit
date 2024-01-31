package upgrader

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/nais/fasit/pkg/upgrader/mocks"
	"github.com/stretchr/testify/assert"
)

var (
	projectId   = "projectId"
	clusterName = "clusterName"
)

func TestClient_GetReleaseChannel(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewUpgrader(t)

	mock.EXPECT().GetReleaseChannel(ctx, projectId, clusterName).Return("STABLE", nil)
	channel, err := mock.GetReleaseChannel(ctx, projectId, clusterName)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	assert.Equal(t, "STABLE", channel)
}

func TestClient_GetRunningOperations(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewUpgrader(t)
	operations := []*containerpb.Operation{
		{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", projectId, clusterName),
			Detail:        "testSuite",
		},
	}

	mock.EXPECT().GetRunningOperations(ctx, projectId, clusterName).Return(operations, nil)
	ops, err := mock.GetRunningOperations(ctx, projectId, clusterName)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	assert.Equal(t, 1, len(ops))
	assert.Equal(t, "operation", ops[0].Name)
	assert.Equal(t, containerpb.Operation_RUNNING, ops[0].Status)
	assert.Equal(t, containerpb.Operation_UPGRADE_NODES, ops[0].OperationType)
	assert.Equal(t, "testSuite", ops[0].Detail)
}

func TestClient_GetAvailableVersions(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewUpgrader(t)
	versions := []string{"1.18.17-gke.1900", "1.19.9-gke.1900", "1.20.5-gke.1900"}

	mock.EXPECT().GetAvailableVersions(ctx, projectId, clusterName, "STABLE").Return(versions, nil)
	availableVersions, err := mock.GetAvailableVersions(ctx, projectId, clusterName, "STABLE")
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	assert.Equal(t, 3, len(availableVersions))
	assert.Equal(t, "1.18.17-gke.1900", availableVersions[0])
	assert.Equal(t, "1.19.9-gke.1900", availableVersions[1])
	assert.Equal(t, "1.20.5-gke.1900", availableVersions[2])
}
