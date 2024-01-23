package upgrader

import (
	"context"
	"fmt"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	version "github.com/hashicorp/go-version"
)

type Client struct {
	client *container.ClusterManagerClient
}

func New(ctx context.Context) (*Client, error) {
	cmClient, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		return &Client{}, err
	}

	return &Client{
		client: cmClient,
	}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) GetRunningOperations(ctx context.Context, projectId, clusterName string) ([]*containerpb.Operation, error) {
	var runningOps []*containerpb.Operation
	parent := c.getParent(projectId)

	operations, err := c.client.ListOperations(ctx, &containerpb.ListOperationsRequest{
		Parent: parent,
	})
	if err != nil {
		return nil, err
	}

	if operations.Operations != nil {
		for _, op := range operations.Operations {
			if op.Status == containerpb.Operation_RUNNING {
				fmt.Println(op.Name)
				runningOps = append(runningOps, op)
			}
		}
	}
	return runningOps, nil
}

func (c *Client) GetOperation(ctx context.Context, projectId, operationId string) (*containerpb.Operation, error) {
	return c.client.GetOperation(ctx, &containerpb.GetOperationRequest{
		Name: fmt.Sprintf("projects/%s/locations/europe-north1/operations/%s", projectId, operationId),
	})
}

func (c *Client) GetReleaseChannel(ctx context.Context, projectId, clusterName string) (string, error) {
	cluster, err := c.getCluster(ctx, projectId, clusterName)
	if err != nil {
		fmt.Println("getRelChan: error", err)
		return "", err
	}
	return cluster.ReleaseChannel.Channel.String(), nil
}

func (c *Client) GetAvailableVersions(ctx context.Context, projectId, clusterName, releaseChannel string) ([]string, error) {
	config, err := c.getServerConfig(ctx, projectId, clusterName)
	if err != nil {
		return nil, err
	}

	currentMasterVer, err := c.GetCurrentMasterVersion(ctx, projectId, clusterName)
	if err != nil {
		return nil, err
	}

	masterVersionObj, err := version.NewVersion(currentMasterVer)
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, channel := range config.Channels {
		if channel.Channel.String() != releaseChannel {
			continue
		}
		index := -1
		for _, v := range channel.ValidVersions {
			versionObj, err := version.NewVersion(v)
			if err != nil {
				return nil, err
			}
			if versionObj.GreaterThanOrEqual(masterVersionObj) {
				index++
			}

		}
		versions = append(versions, channel.ValidVersions[0:index]...)
	}
	return versions, nil
}

func (c *Client) UpgradeMaster(ctx context.Context, projectId, clusterName, version string) (*containerpb.Operation, error) {
	return c.client.UpdateMaster(ctx, &containerpb.UpdateMasterRequest{
		Name:          c.getName(projectId, clusterName),
		MasterVersion: version,
	})
}

func (c *Client) UpgradeNodePool(ctx context.Context, projectId, clusterName, nodePoolName, version string) (*containerpb.Operation, error) {
	return c.client.UpdateNodePool(ctx, &containerpb.UpdateNodePoolRequest{
		Name:        c.getNodePoolName(projectId, clusterName, nodePoolName),
		NodeVersion: version,
	})
}

func (c *Client) Upgrade(ctx context.Context, projectId, clusterName, version string) error {
	_, err := c.client.UpdateCluster(ctx, &containerpb.UpdateClusterRequest{
		Name:      c.getName(projectId, clusterName),
		Update:    &containerpb.ClusterUpdate{DesiredMasterVersion: version},
		ProjectId: projectId,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetCurrentMasterVersion(ctx context.Context, projectId, clusterName string) (string, error) {
	cluster, err := c.getCluster(ctx, projectId, clusterName)
	if err != nil {
		return "", err
	}
	return cluster.CurrentMasterVersion, nil
}

func (c *Client) GetNodePools(ctx context.Context, projectId, clusterName string) ([]*containerpb.NodePool, error) {
	cluster, err := c.getCluster(ctx, projectId, clusterName)
	if err != nil {
		return nil, err
	}
	return cluster.NodePools, nil
}

func (c *Client) getServerConfig(ctx context.Context, projectId, clusterName string) (*containerpb.ServerConfig, error) {
	config, err := c.client.GetServerConfig(ctx, &containerpb.GetServerConfigRequest{
		Name: c.getName(projectId, clusterName),
	})
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Client) getCluster(ctx context.Context, projectId, clusterName string) (*containerpb.Cluster, error) {
	cluster, err := c.client.GetCluster(ctx, &containerpb.GetClusterRequest{
		Name: c.getName(projectId, clusterName),
	})
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

func (c *Client) getNodePoolName(projectId, clusterName, nodePoolName string) string {
	return "projects/" + projectId + "/locations/europe-north1/clusters/" + clusterName + "/nodePools/" + nodePoolName
}

func (c *Client) getName(projectId, clusterName string) string {
	return "projects/" + projectId + "/locations/europe-north1/clusters/" + clusterName
}

func (c *Client) getParent(projectId string) string {
	return "projects/" + projectId + "/locations/europe-north1"
}
