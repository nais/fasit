package upgrader

import (
	"context"
	"fmt"
	"strings"
	"time"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	version "github.com/hashicorp/go-version"
	"github.com/nais/fasit/internal/graph/model"
)

type Upgrader interface {
	GetReleaseChannel(ctx context.Context, projectID string, environment *model.Environment) (string, error)
	GetCurrentControlPlaneVersion(ctx context.Context, projectID string, environment *model.Environment) (string, error)
	GetAvailableVersions(ctx context.Context, projectID string, environment *model.Environment, releaseChannel string) ([]string, error)
	GetRunningOperations(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.Operation, error)
	UpgradeControlPlane(ctx context.Context, projectID string, environment *model.Environment, version string) (*containerpb.Operation, error)
	UpgradeNodePool(ctx context.Context, projectID string, environment *model.Environment, nodePoolName, version string) (*containerpb.Operation, error)
	GetNodePools(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.NodePool, error)
	GetOperation(ctx context.Context, projectID, operationID string) (*containerpb.Operation, error)
	IsTimeInRange(start, end int) bool
}

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

func (c *Client) IsTimeInRange(start, end int) bool {
	location, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return false
	}

	upgradeWindowStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), start, 0, 0, 0, location)
	upgradeWindowEnd := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), end, 0, 0, 0, location)

	if time.Now().Before(upgradeWindowStart) || time.Now().After(upgradeWindowEnd) {
		return false
	}

	return true
}

func (c *Client) GetRunningOperations(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.Operation, error) {
	var runningOps []*containerpb.Operation
	parent := c.getParent(projectID)

	operations, err := c.client.ListOperations(ctx, &containerpb.ListOperationsRequest{
		Parent: parent,
	})
	if err != nil {
		return nil, err
	}

	clusterName := c.getClusterName(environment)

	for _, op := range operations.Operations {
		if strings.Contains(op.TargetLink, clusterName) && op.Status == containerpb.Operation_RUNNING {
			runningOps = append(runningOps, op)
		}
	}
	return runningOps, nil
}

func (c *Client) GetOperation(ctx context.Context, projectID, operationID string) (*containerpb.Operation, error) {
	return c.client.GetOperation(ctx, &containerpb.GetOperationRequest{
		Name: fmt.Sprintf("projects/%s/locations/europe-north1/operations/%s", projectID, operationID),
	})
}

func (c *Client) GetReleaseChannel(ctx context.Context, projectID string, environment *model.Environment) (string, error) {
	cluster, err := c.getCluster(ctx, projectID, environment)
	if err != nil {
		return "", err
	}
	return cluster.ReleaseChannel.Channel.String(), nil
}

func (c *Client) GetAvailableVersions(ctx context.Context, projectID string, environment *model.Environment, releaseChannel string) ([]string, error) {
	config, err := c.getServerConfig(ctx, projectID, environment)
	if err != nil {
		return nil, err
	}

	currentControlPlaneVer, err := c.GetCurrentControlPlaneVersion(ctx, projectID, environment)
	if err != nil {
		return nil, err
	}

	controlPlaneVersionObj, err := version.NewVersion(currentControlPlaneVer)
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, channel := range config.Channels {
		if channel.Channel.String() != releaseChannel {
			continue
		}
		index := 0
		for _, v := range channel.ValidVersions {
			versionObj, err := version.NewVersion(v)
			if err != nil {
				return nil, err
			}
			if versionObj.GreaterThan(controlPlaneVersionObj) {
				index++
			}

		}
		if index == 0 {
			return nil, nil
		}
		versions = append(versions, channel.ValidVersions[0:index]...)
	}
	return versions, nil
}

func (c *Client) UpgradeControlPlane(ctx context.Context, projectID string, environment *model.Environment, version string) (*containerpb.Operation, error) {
	clusterName := c.getClusterName(environment)
	return c.client.UpdateMaster(ctx, &containerpb.UpdateMasterRequest{
		Name:          c.getName(projectID, clusterName),
		MasterVersion: version,
	})
}

func (c *Client) UpgradeNodePool(ctx context.Context, projectID string, environment *model.Environment, nodePoolName, version string) (*containerpb.Operation, error) {
	clusterName := c.getClusterName(environment)
	return c.client.UpdateNodePool(ctx, &containerpb.UpdateNodePoolRequest{
		Name:        c.getNodePoolName(projectID, clusterName, nodePoolName),
		NodeVersion: version,
	})
}

func (c *Client) GetCurrentControlPlaneVersion(ctx context.Context, projectID string, environment *model.Environment) (string, error) {
	cluster, err := c.getCluster(ctx, projectID, environment)
	if err != nil {
		return "", err
	}
	return cluster.CurrentMasterVersion, nil
}

func (c *Client) GetNodePools(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.NodePool, error) {
	cluster, err := c.getCluster(ctx, projectID, environment)
	if err != nil {
		return nil, err
	}
	return cluster.NodePools, nil
}

func (c *Client) getServerConfig(ctx context.Context, projectID string, environment *model.Environment) (*containerpb.ServerConfig, error) {
	clusterName := c.getClusterName(environment)
	return c.client.GetServerConfig(ctx, &containerpb.GetServerConfigRequest{
		Name: c.getName(projectID, clusterName),
	})
}

func (c *Client) getCluster(ctx context.Context, projectID string, environment *model.Environment) (*containerpb.Cluster, error) {
	clusterName := c.getClusterName(environment)
	return c.client.GetCluster(ctx, &containerpb.GetClusterRequest{
		Name: c.getName(projectID, clusterName),
	})
}

func (c *Client) getNodePoolName(projectID, clusterName, nodePoolName string) string {
	return "projects/" + projectID + "/locations/europe-north1/clusters/" + clusterName + "/nodePools/" + nodePoolName
}

func (c *Client) getName(projectID, clusterName string) string {
	return "projects/" + projectID + "/locations/europe-north1/clusters/" + clusterName
}

func (c *Client) getParent(projectID string) string {
	return "projects/" + projectID + "/locations/europe-north1"
}

func (c *Client) getClusterName(environment *model.Environment) string {
	if environment.Kind == model.EnvironmentKindTenant {
		return "nais-" + environment.Name
	}
	if environment.Kind == model.EnvironmentKindManagement {
		return "nais-" + environment.Name + "-v2"
	}
	return environment.Name
}
