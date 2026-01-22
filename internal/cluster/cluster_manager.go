package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	version "github.com/hashicorp/go-version"
	"github.com/nais/fasit/internal/graph/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ClusterManager interface {
	GetReleaseChannel(ctx context.Context, projectID string, environment *model.Environment) (string, error)
	GetCurrentControlPlaneVersion(ctx context.Context, projectID string, environment *model.Environment) (string, error)
	GetAvailableVersions(ctx context.Context, projectID string, environment *model.Environment, releaseChannel string) ([]string, error)
	GetRunningOperations(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.Operation, error)
	UpgradeControlPlane(ctx context.Context, projectID string, environment *model.Environment, version string) (*containerpb.Operation, error)
	UpgradeNodePool(ctx context.Context, projectID string, environment *model.Environment, nodePoolName, version string) (*containerpb.Operation, error)
	GetNodePools(ctx context.Context, projectID string, environment *model.Environment) ([]*containerpb.NodePool, error)
	GetOperation(ctx context.Context, projectID, operationID string) (*containerpb.Operation, error)
	SetMaintenanceWindow(ctx context.Context, projectID string, environment *model.Environment, window *model.MaintenanceWindow) (*containerpb.Operation, error)
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

// IsBusinessHours checks if the current time is within business hours (Mon-Fri, 9 AM - 4 PM Europe/Oslo)
func IsBusinessHours() bool {
	location, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return false
	}

	now := time.Now().In(location)

	// Check if it's a weekday (Monday-Friday)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// Check if it's within business hours (9 AM - 4 PM)
	hour := now.Hour()
	if hour < 9 || hour >= 16 {
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
		if strings.Contains(op.TargetLink, clusterName) && (op.Status == containerpb.Operation_RUNNING || op.Status == containerpb.Operation_PENDING) {
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

// SetMaintenanceWindow configures the maintenance window for a GKE cluster.
// Pass nil to remove the window and allow GKE to perform maintenance anytime.
func (c *Client) SetMaintenanceWindow(ctx context.Context, projectID string, environment *model.Environment, window *model.MaintenanceWindow) (*containerpb.Operation, error) {
	clusterName := c.getClusterName(environment)
	clusterPath := c.getName(projectID, clusterName)

	cluster, err := c.client.GetCluster(ctx, &containerpb.GetClusterRequest{
		Name: clusterPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	var resourceVersion string
	if cluster.MaintenancePolicy != nil {
		resourceVersion = cluster.MaintenancePolicy.ResourceVersion
	}

	if window == nil {
		return c.client.SetMaintenancePolicy(ctx, &containerpb.SetMaintenancePolicyRequest{
			Name: clusterPath,
			MaintenancePolicy: &containerpb.MaintenancePolicy{
				ResourceVersion: resourceVersion,
			},
		})
	}

	var recurringWindow *containerpb.RecurringTimeWindow

	startParts := strings.Split(window.StartTime, ":")
	endParts := strings.Split(window.EndTime, ":")
	if len(startParts) != 2 || len(endParts) != 2 {
		return nil, fmt.Errorf("invalid time format, expected HH:MM")
	}

	var startHour, startMin, endHour, endMin int
	if _, err := fmt.Sscanf(window.StartTime, "%d:%d", &startHour, &startMin); err != nil {
		return nil, fmt.Errorf("invalid start time format: %w", err)
	}
	if _, err := fmt.Sscanf(window.EndTime, "%d:%d", &endHour, &endMin); err != nil {
		return nil, fmt.Errorf("invalid end time format: %w", err)
	}

	if startHour < 0 || startHour > 23 || endHour < 0 || endHour > 23 {
		return nil, fmt.Errorf("hours must be between 0 and 23")
	}
	if startMin < 0 || startMin > 59 || endMin < 0 || endMin > 59 {
		return nil, fmt.Errorf("minutes must be between 0 and 59")
	}

	// Create times in UTC
	now := time.Now().UTC()
	startTime := time.Date(now.Year(), now.Month(), now.Day(),
		startHour, startMin, 0, 0, time.UTC)
	endTime := time.Date(now.Year(), now.Month(), now.Day(),
		endHour, endMin, 0, 0, time.UTC)

	if len(window.Days) == 0 {
		return nil, fmt.Errorf("at least one day must be specified for the maintenance window")
	}

	days := make([]string, len(window.Days))
	dayMap := map[model.DayOfWeek]string{
		model.DayOfWeekMonday:    "MO",
		model.DayOfWeekTuesday:   "TU",
		model.DayOfWeekWednesday: "WE",
		model.DayOfWeekThursday:  "TH",
		model.DayOfWeekFriday:    "FR",
		model.DayOfWeekSaturday:  "SA",
		model.DayOfWeekSunday:    "SU",
	}
	for i, day := range window.Days {
		days[i] = dayMap[day]
	}
	recurrence := "FREQ=WEEKLY;BYDAY=" + strings.Join(days, ",")

	recurringWindow = &containerpb.RecurringTimeWindow{
		Window: &containerpb.TimeWindow{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		Recurrence: recurrence,
	}

	policy := &containerpb.MaintenancePolicy{
		ResourceVersion: resourceVersion,
		Window: &containerpb.MaintenanceWindow{
			Policy: &containerpb.MaintenanceWindow_RecurringWindow{
				RecurringWindow: recurringWindow,
			},
		},
	}

	return c.client.SetMaintenancePolicy(ctx, &containerpb.SetMaintenancePolicyRequest{
		Name:              clusterPath,
		MaintenancePolicy: policy,
	})
}
