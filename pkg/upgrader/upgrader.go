package upgrader

import (
	"context"
	"fmt"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
)

type Client struct {
	client *container.ClusterManagerClient
}

type Upgrader interface {
	GetReleaseChannel(ctx context.Context, projectId, clusterName string) (string, error)
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

func (c *Client) GetReleaseChannel(ctx context.Context, projectId, clusterName string) (string, error) {
	cluster, err := c.getCluster(ctx, projectId, clusterName)
	if err != nil {
		fmt.Println("getRelChan: error", err)
		return "", err
	}
	return cluster.ReleaseChannel.Channel.String(), nil
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

func (c *Client) getName(projectId, clusterName string) string {
	return "projects/" + projectId + "/locations/europe-north1/clusters/" + clusterName
}
