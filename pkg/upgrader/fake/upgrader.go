package fake

import (
	"context"
	"fmt"

	"cloud.google.com/go/container/apiv1/containerpb"
)

type Upgrader struct{}

func NewUpgrader() *Upgrader {
	return &Upgrader{}
}

func (c *Upgrader) GetReleaseChannel(ctx context.Context, projectId, clusterName string) (string, error) {
	return "STABLE", nil
}

func (c *Upgrader) GetCurrentMasterVersion(ctx context.Context, projectId, clusterName string) (string, error) {
	return "1.27.4-gke.900", nil
}

func (c *Upgrader) GetAvailableVersions(ctx context.Context, projectId, clusterName, releaseChannel string) ([]string, error) {
	return []string{"1.28.3-gke.1203001", "1.27.7-gke.1121000", "1.27.5-gke.200", "1.27.4-gke.900", "1.27.3-gke.100"}, nil
}

func (c *Upgrader) Upgrade(ctx context.Context, projectId, clusterName, version string) error {
	fmt.Println("UpgradeK8sVersion", projectId, clusterName, version)
	return nil
}

func (c *Upgrader) GetRunningOperations(ctx context.Context, projectId, clusterName string) ([]*containerpb.Operation, error) {
	ret := []*containerpb.Operation{}
	if clusterName == "dev-gcp" && projectId == "nais-dev-gcp" {
		fmt.Println("GetRunningOperations", projectId, clusterName)
		operation := &containerpb.Operation{
			Name:          "operation-1705388564221-30db27fc-fd46-4b7c-b8ba-50be2adfe2c2",
			Zone:          "europe-north1",
			OperationType: containerpb.Operation_UPGRADE_NODES,
			Status:        containerpb.Operation_RUNNING,
			Detail:        "Updating nap-e2-standard-16-1x43ive2, done with 18 out of 44 nodes (40.9%): 2 being processed, 6 succeeded",
			SelfLink:      "https://container.googleapis.com/v1/projects/182271809372/locations/europe-north1/operations/operation-1705388564221-30db27fc-fd46-4b7c-b8ba-50be2adfe2c2",
			TargetLink:    "https://container.googleapis.com/v1/projects/182271809372/locations/europe-north1/clusters/dev-gcp/nodePools/nap-e2-standard-16-1x43ive2",
			StartTime:     "2024-01-16T07:02:44.221595882Z",
			Progress: &containerpb.OperationProgress{
				Metrics: []*containerpb.OperationProgress_Metric{
					{
						Name:  "NODES_TOTAL",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 44},
					},
					{
						Name:  "NODES_FAILED",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 0},
					},
					{
						Name:  "NODES_COMPLETE",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 18},
					},
					{
						Name:  "NODES_DONE",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 18},
					},
					{
						Name:  "NODE_PDB_DELAY_SECONDS",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 14395},
					},
				},
			},
		}
		ret = append(ret, operation)
	}
	return ret, nil
}
