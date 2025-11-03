package upgrader

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/upgrader/mocks"
)

var (
	projectID   = "projectId"
	clusterName = "clusterName"
	environment = model.Environment{
		ID:       uuid.New(),
		Name:     "t1",
		Kind:     model.EnvironmentKindTenant,
		TenantID: uuid.New(),
	}
)

func TestClient_GetReleaseChannel(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewClusterManager(t)

	mock.EXPECT().GetReleaseChannel(ctx, projectID, &environment).Return("STABLE", nil)
	channel, err := mock.GetReleaseChannel(ctx, projectID, &environment)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if channel != "STABLE" {
		t.Errorf("got %s, want STABLE", channel)
	}
}

func TestClient_GetRunningOperations(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewClusterManager(t)
	operations := []*containerpb.Operation{
		{
			Name:          "operation",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
			TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/%s", projectID, clusterName),
			Detail:        "testSuite",
		},
	}

	mock.EXPECT().GetRunningOperations(ctx, projectID, &environment).Return(operations, nil)
	ops, err := mock.GetRunningOperations(ctx, projectID, &environment)
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	if ops == nil {
		t.Errorf("got nil, want operations")
	}
	if len(ops) != 1 {
		t.Errorf("got %d, want 1", len(ops))
	}
	if containerpb.Operation_RUNNING != ops[0].Status {
		t.Errorf("got %s, want RUNNING", ops[0].Status)
	}
	if containerpb.Operation_UPGRADE_NODES != ops[0].OperationType {
		t.Errorf("got %s, want UPGRADE_NODES", ops[0].OperationType)
	}
}

func TestClient_GetAvailableVersions(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewClusterManager(t)
	versions := []string{"1.18.17-gke.1900", "1.19.9-gke.1900", "1.20.5-gke.1900"}

	mock.EXPECT().GetAvailableVersions(ctx, projectID, &environment, "STABLE").Return(versions, nil)
	availableVersions, err := mock.GetAvailableVersions(ctx, projectID, &environment, "STABLE")
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}

	if availableVersions == nil {
		t.Errorf("got nil, want versions")
	}
	if len(availableVersions) != 3 {
		t.Errorf("got %d, want 3", len(availableVersions))
	}
}
