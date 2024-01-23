package fake

import (
	"context"
	"fmt"

	"cloud.google.com/go/container/apiv1/containerpb"
	version "github.com/hashicorp/go-version"
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
	currentMasterVersion, _ := c.GetCurrentMasterVersion(ctx, projectId, clusterName)
	masterVersionObj, err := version.NewVersion(currentMasterVersion)
	if err != nil {
		return nil, err
	}

	availableVersions := []string{"1.28.3-gke.1203001", "1.27.7-gke.1121000", "1.27.5-gke.200", "1.27.4-gke.900", "1.27.3-gke.100"}

	var versions []string
	index := -1
	for _, v := range availableVersions {
		versionObj, err := version.NewVersion(v)
		if err != nil {
			return nil, err
		}
		if versionObj.GreaterThanOrEqual(masterVersionObj) {
			index++
		}

	}
	versions = append(versions, availableVersions[0:index]...)

	return versions, nil
}

func (c *Upgrader) Upgrade(ctx context.Context, projectId, clusterName, version string) error {
	fmt.Println("UpgradeK8sVersion", projectId, clusterName, version)
	return nil
}

func (c *Upgrader) UpgradeMaster(ctx context.Context, projectId, clusterName, version string) (*containerpb.Operation, error) {
	ret := containerpb.Operation{
		Name:          "operation-1704958496609-d23dbf0f-fb3c-46e1-80f6-7922d321ddee",
		Zone:          "europe-north1",
		OperationType: containerpb.Operation_UPGRADE_MASTER,
		Status:        containerpb.Operation_RUNNING,
		StartTime:     "2024-01-11T07:34:56.609426745Z",
		SelfLink:      "https://container.googleapis.com/v1/projects/501288000449/locations/europe-north1/operations/operation-1704958496609-d23dbf0f-fb3c-46e1-80f6-7922d321ddee",
		TargetLink:    "https://container.googleapis.com/v1/projects/501288000449/locations/europe-north1/clusters/ci-gcp",
	}

	return &ret, nil
}

func (c *Upgrader) UpgradeNodePool(ctx context.Context, projectId, clusterName, nodePoolName, version string) (*containerpb.Operation, error) {
	ret := containerpb.Operation{}
	fmt.Println("Upgrade nodepool", projectId, clusterName, nodePoolName, version)

	return &ret, nil
}

func (c *Upgrader) GetNodePools(ctx context.Context, projectId, clusterName string) ([]*containerpb.NodePool, error) {
	return []*containerpb.NodePool{
		{
			Name: "nap-e2-standard-16-1x43ive2",
		},
	}, nil
}

func (c *Upgrader) GetRunningOperations(ctx context.Context, projectId, clusterName string) ([]*containerpb.Operation, error) {
	fmt.Printf("GetRunningOperations %s %s\n", projectId, clusterName)
	ret := []*containerpb.Operation{}
	if clusterName == "dev" && projectId == "nais-dev" {
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
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 19},
					},
					{
						Name:  "NODES_DONE",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 19},
					},
					{
						Name:  "NODE_PDB_DELAY_SECONDS",
						Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 14145},
					},
				},
			},
		}
		ret = append(ret, operation)
	}
	return ret, nil
}

func (c *Upgrader) GetOperation(ctx context.Context, projectId, operationId string) (*containerpb.Operation, error) {
	return &containerpb.Operation{
		Name:          operationId,
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
					Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 19},
				},
				{
					Name:  "NODES_DONE",
					Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 19},
				},
				{
					Name:  "NODE_PDB_DELAY_SECONDS",
					Value: &containerpb.OperationProgress_Metric_IntValue{IntValue: 6969696},
				},
			},
		},
	}, nil
}
