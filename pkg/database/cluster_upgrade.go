package database

import (
	"context"
	"errors"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/pkg/database/gensql"

	"github.com/nais/fasit/pkg/graph/model"
)

type ClusterUpgraderRepo interface {
	CreateOrUpdateClusterOperation(ctx context.Context, tenantId, envId, versionId uuid.UUID, op *containerpb.Operation) (*model.ClusterOperation, error)
	GetRunningClusterOperations(ctx context.Context, tenantId, envId uuid.UUID) ([]*model.ClusterOperation, error)
	CreateClusterUpgrade(ctx context.Context, tenantId, envId uuid.UUID, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGet(ctx context.Context, tenantId, envId uuid.UUID) (*model.ClusterUpgradeStatus, error)
	UpdateClusterUpgradeStatus(ctx context.Context, tenantId, envId uuid.UUID, status gensql.ClusterUpgradesStatus, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error)
}

func clusterUpgradeFromSQL(p gensql.ClusterUpgrade) *model.ClusterUpgradeStatus {
	return &model.ClusterUpgradeStatus{
		ID:            p.ID,
		Version:       p.Version,
		UpgradeStatus: model.UpgradeStatus(p.Status),
		LastModified:  p.LastModified.Time,
	}
}

func clusterOperationFromSQL(p gensql.ClusterOperation) *model.ClusterOperation {
	return &model.ClusterOperation{
		ID:                  p.ID,
		TenantID:            p.TenantID,
		EnvironmentID:       p.EnvironmentID,
		UpgradeID:           p.UpgradeID,
		Status:              p.Status,
		Type:                p.Type,
		NodesTotal:          int(p.NodesTotal),
		NodesFailed:         int(p.NodesFailed),
		NodesCompleted:      int(p.NodesCompleted),
		NodesDone:           int(p.NodesDone),
		NodePdbDelaySeconds: int(p.NodePdbDelaySeconds),
		StartTime:           p.StartTime.Time,
		LastModified:        p.LastModified.Time,
	}
}

func (r *repo) ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesGetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterVersion), nil
}

func (r *repo) UpdateClusterUpgradeStatus(ctx context.Context, tenantId, envId uuid.UUID, status gensql.ClusterUpgradesStatus, version string) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesUpdateStatus(ctx, gensql.ClusterUpgradesUpdateStatusParams{
		Status:   status,
		Tenantid: tenantId,
		Envid:    envId,
		Version:  version,
	})
	if err != nil {
		return nil, err
	}
	if clusterVersion.ID == uuid.Nil {
		return nil, nil
	}
	return clusterUpgradeFromSQL(clusterVersion), nil
}

func (r *repo) ClusterUpgradeGet(ctx context.Context, tenantId, envId uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesGet(ctx, gensql.ClusterUpgradesGetParams{
		Tenantid: tenantId,
		Envid:    envId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterVersion), nil
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

func (r *repo) GetRunningClusterOperations(ctx context.Context, tenantId, envId uuid.UUID) ([]*model.ClusterOperation, error) {
	ops, err := r.querier.ClusterOperationsGet(ctx, gensql.ClusterOperationsGetParams{
		Tenantid: tenantId,
		Envid:    envId,
		Status:   "RUNNING",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}

	var ret []*model.ClusterOperation
	for _, op := range ops {
		ret = append(ret, clusterOperationFromSQL(op))
	}

	return ret, nil
}

func (r *repo) CreateOrUpdateClusterOperation(ctx context.Context, tenantId, envId, upgradeId uuid.UUID, op *containerpb.Operation) (*model.ClusterOperation, error) {
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

	co, err := r.querier.ClusterOperationCreateOrUpdate(ctx, gensql.ClusterOperationCreateOrUpdateParams{
		ID:                  op.Name,
		Status:              op.Status.String(),
		TenantID:            tenantId,
		EnvID:               envId,
		UpgradeID:           upgradeId,
		Type:                op.OperationType.String(),
		NodesTotal:          int32(nodes_total),
		NodesFailed:         int32(nodes_failed),
		NodesCompleted:      int32(nodes_complete),
		NodesDone:           int32(nodes_done),
		NodePdbDelaySeconds: int32(node_pdb_delay_seconds),
	})
	if err != nil {
		return &model.ClusterOperation{}, err
	}
	return clusterOperationFromSQL(co), nil
}
