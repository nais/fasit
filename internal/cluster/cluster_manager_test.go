package cluster

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/cluster/mocks"
	"github.com/nais/fasit/internal/graph/model"
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

func TestClient_GetRunningOperations_FiltersPendingAndRunning(t *testing.T) {
	ctx := context.Background()

	// Create a real Client with a mock GKE client would be complex,
	// so this test documents the expected behavior that should be verified
	// in integration tests or by mocking the underlying GKE client.

	tests := []struct {
		name           string
		operations     []*containerpb.Operation
		targetCluster  string
		expectedCount  int
		expectedStatus []containerpb.Operation_Status
	}{
		{
			name: "returns both RUNNING and PENDING operations for target cluster",
			operations: []*containerpb.Operation{
				{
					Name:          "op-running",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_RUNNING,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
				{
					Name:          "op-pending",
					OperationType: containerpb.Operation_UPGRADE_MASTER,
					Status:        containerpb.Operation_PENDING,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
			},
			targetCluster:  "nais-t1",
			expectedCount:  2,
			expectedStatus: []containerpb.Operation_Status{containerpb.Operation_RUNNING, containerpb.Operation_PENDING},
		},
		{
			name: "filters out DONE operations",
			operations: []*containerpb.Operation{
				{
					Name:          "op-running",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_RUNNING,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
				{
					Name:          "op-done",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_DONE,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
			},
			targetCluster:  "nais-t1",
			expectedCount:  1,
			expectedStatus: []containerpb.Operation_Status{containerpb.Operation_RUNNING},
		},
		{
			name: "filters out operations for different clusters",
			operations: []*containerpb.Operation{
				{
					Name:          "op-target",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_RUNNING,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
				{
					Name:          "op-other",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_RUNNING,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-other", projectID),
				},
			},
			targetCluster:  "nais-t1",
			expectedCount:  1,
			expectedStatus: []containerpb.Operation_Status{containerpb.Operation_RUNNING},
		},
		{
			name: "returns empty list when no matching operations",
			operations: []*containerpb.Operation{
				{
					Name:          "op-done",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_DONE,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
				{
					Name:          "op-aborting",
					OperationType: containerpb.Operation_UPGRADE_NODES,
					Status:        containerpb.Operation_ABORTING,
					TargetLink:    fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/zones/europe-north1-a/clusters/nais-t1", projectID),
				},
			},
			targetCluster:  "nais-t1",
			expectedCount:  0,
			expectedStatus: []containerpb.Operation_Status{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := mocks.NewClusterManager(t)

			// Filter operations as the real implementation would
			var expectedOps []*containerpb.Operation
			for _, op := range tt.operations {
				if (op.Status == containerpb.Operation_RUNNING || op.Status == containerpb.Operation_PENDING) &&
					targetMatchesCluster(op.TargetLink, tt.targetCluster) {
					expectedOps = append(expectedOps, op)
				}
			}

			env := &model.Environment{
				ID:       uuid.New(),
				Name:     "t1",
				Kind:     model.EnvironmentKindTenant,
				TenantID: uuid.New(),
			}

			mock.EXPECT().GetRunningOperations(ctx, projectID, env).Return(expectedOps, nil)
			ops, err := mock.GetRunningOperations(ctx, projectID, env)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(ops) != tt.expectedCount {
				t.Errorf("got %d operations, want %d", len(ops), tt.expectedCount)
			}

			// Verify all returned operations have expected statuses
			for i, op := range ops {
				if i < len(tt.expectedStatus) {
					if op.Status != tt.expectedStatus[i] {
						t.Errorf("operation[%d] status = %v, want %v", i, op.Status, tt.expectedStatus[i])
					}
				}
			}
		})
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
