package database

import (
	"context"
	"errors"
	"strings"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type ClusterUpgraderRepo interface {
	CreateOrUpdateClusterOperation(ctx context.Context, tenantId, envId, versionId uuid.UUID, op *containerpb.Operation) (*model.EnvironmentOperation, error)
	GetRunningClusterOperation(ctx context.Context, tenantId, envId uuid.UUID) (*model.EnvironmentOperation, error)
	CreateClusterUpgrade(ctx context.Context, tenantId, envId uuid.UUID, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGet(ctx context.Context, tenantId, envId uuid.UUID) (*model.ClusterUpgradeStatus, error)
	UpdateClusterUpgradeStatus(ctx context.Context, tenantId, envId uuid.UUID, status gensql.ClusterUpgradesStatus, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error)
	ClusterOperationsGetByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentOperation, error)
	ClusterOperationsGetByUpgradeID(ctx context.Context, upgradeId uuid.UUID) ([]*model.EnvironmentOperation, error)
	SetClusterUpgradesSlackMessage(ctx context.Context, id uuid.UUID, slackMessageTs, channelID string) (*model.ClusterUpgradeStatus, error)
}

func clusterUpgradeFromSQL(p gensql.ClusterUpgrade) *model.ClusterUpgradeStatus {
	return &model.ClusterUpgradeStatus{
		ID:                    p.ID,
		Version:               p.Version,
		UpgradeStatus:         model.UpgradeStatus(p.Status),
		LastModified:          p.LastModified.Time,
		StartTime:             p.StartTime.Time,
		EnvironmentID:         p.EnvironmentID,
		SlackMessageTimestamp: p.SlackMessageTimestamp.String,
		SlackChannelID:        p.SlackChannelID.String,
	}
}

func clusterOperationFromSQL(p gensql.ClusterOperation) *model.EnvironmentOperation {
	return &model.EnvironmentOperation{
		ID:                  p.ID,
		Name:                p.OperationName,
		Status:              p.Status,
		Type:                p.Type,
		Target:              p.Target,
		Detail:              p.Detail,
		NodesTotal:          int(p.NodesTotal),
		NodesFailed:         int(p.NodesFailed),
		NodesCompleted:      int(p.NodesCompleted),
		NodesDone:           int(p.NodesDone),
		NodePdbDelaySeconds: int(p.NodePdbDelaySeconds),
		StartTime:           p.StartTime.Time,
		LastModified:        p.LastModified.Time,
	}
}

func (r *repo) ClusterOperationsGetByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentOperation, error) {
	clusterOperation, err := r.querier.ClusterOperationsGetByID(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if clusterOperation.EnvironmentID == uuid.Nil {
		return &model.EnvironmentOperation{}, nil
	}
	return clusterOperationFromSQL(clusterOperation), nil
}

func (r *repo) ClusterOperationsGetByUpgradeID(ctx context.Context, upgradeId uuid.UUID) ([]*model.EnvironmentOperation, error) {
	clusterOperations, err := r.querier.ClusterOperationsGetByUpgradeID(ctx, upgradeId)
	if err != nil {
		return nil, err
	}

	var ops []*model.EnvironmentOperation

	for _, op := range clusterOperations {
		ops = append(ops, clusterOperationFromSQL(op))
	}

	return ops, nil
}

func (r *repo) ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesGetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterVersion), nil
}

func (r *repo) SetClusterUpgradesSlackMessage(ctx context.Context, id uuid.UUID, slackMessageTs, channelID string) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrade, err := r.querier.ClusterUpgradesSetSlackMessage(ctx, gensql.ClusterUpgradesSetSlackMessageParams{
		Slackmessagetimestamp: ptrToNullString(&slackMessageTs),
		Slackchannelid:        ptrToNullString(&channelID),
		ID:                    id,
	})
	if err != nil {
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterUpgrade), nil
}

func (r *repo) UpdateClusterUpgradeStatus(ctx context.Context, tenantId, envId uuid.UUID, status gensql.ClusterUpgradesStatus, version string) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrade, err := r.querier.ClusterUpgradesUpdateStatus(ctx, gensql.ClusterUpgradesUpdateStatusParams{
		Status:   status,
		Tenantid: tenantId,
		Envid:    envId,
		Version:  version,
	})
	if err != nil {
		return nil, err
	}

	return clusterUpgradeFromSQL(clusterUpgrade), nil
}

func (r *repo) ClusterUpgradeGet(ctx context.Context, tenantId, envId uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrades, err := r.querier.ClusterUpgradesGet(ctx, gensql.ClusterUpgradesGetParams{
		Tenantid: tenantId,
		Envid:    envId,
	})
	if err != nil {
		return nil, err
	}

	if len(clusterUpgrades) == 0 {
		return nil, nil
	}

	if len(clusterUpgrades) > 1 {
		return nil, errors.New("found more than one cluster upgrade")
	}
	return clusterUpgradeFromSQL(clusterUpgrades[0]), nil
}

func (r *repo) CreateClusterUpgrade(ctx context.Context, tenantId, envId uuid.UUID, version string) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesCreate(ctx, gensql.ClusterUpgradesCreateParams{
		Tenantid: tenantId,
		Envid:    envId,
		Version:  version,
	})
	if err != nil {
		return nil, err
	}

	return clusterUpgradeFromSQL(clusterVersion), nil
}

func (r *repo) GetRunningClusterOperation(ctx context.Context, tenantId, envId uuid.UUID) (*model.EnvironmentOperation, error) {
	op, err := r.querier.ClusterOperationGet(ctx, gensql.ClusterOperationGetParams{
		Tenantid: tenantId,
		Envid:    envId,
		Status:   "RUNNING",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return clusterOperationFromSQL(op), nil
}

func (r *repo) CreateOrUpdateClusterOperation(ctx context.Context, tenantId, envId, upgradeId uuid.UUID, op *containerpb.Operation) (*model.EnvironmentOperation, error) {
	nodes_total := 0
	nodes_failed := 0
	nodes_complete := 0
	nodes_done := 0
	node_pdb_delay_seconds := 0

	for _, metric := range op.Progress.GetMetrics() {
		if metric.Name == "NODES_TOTAL" {
			nodes_total = int(metric.GetIntValue())
		}
		if metric.Name == "NODES_FAILED" {
			nodes_failed = int(metric.GetIntValue())
		}
		if metric.Name == "NODES_COMPLETE" {
			nodes_complete = int(metric.GetIntValue())
		}
		if metric.Name == "NODES_DONE" {
			nodes_done = int(metric.GetIntValue())
		}
		if metric.Name == "NODE_PDB_DELAY_SECONDS" {
			node_pdb_delay_seconds = int(metric.GetIntValue())
		}
	}

	var id uuid.UUID
	var err error
	if op.Name != "" {
		uu := strings.SplitAfterN(op.Name, "-", 3)[2]
		id, err = uuid.Parse(uu)
		if err != nil {
			return nil, err
		}
	}

	co, err := r.querier.ClusterOperationCreateOrUpdate(ctx, gensql.ClusterOperationCreateOrUpdateParams{
		ID:                  id,
		OperationName:       op.Name,
		Status:              op.Status.String(),
		TenantID:            tenantId,
		EnvID:               envId,
		UpgradeID:           upgradeId,
		Type:                op.OperationType.String(),
		Target:              op.TargetLink,
		Detail:              op.Detail,
		NodesTotal:          int32(nodes_total),
		NodesFailed:         int32(nodes_failed),
		NodesCompleted:      int32(nodes_complete),
		NodesDone:           int32(nodes_done),
		NodePdbDelaySeconds: int32(node_pdb_delay_seconds),
	})
	if err != nil {
		return &model.EnvironmentOperation{}, err
	}
	return clusterOperationFromSQL(co), nil
}
