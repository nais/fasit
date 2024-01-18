package database

import (
	"context"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
)

type ClusterUpgraderRepo interface {
	ClusterOperationCreateOrUpdate(ctx context.Context, tenantId, envId uuid.UUID, masterVersion string, op *containerpb.Operation) (gensql.ClusterUpgrade, error)
}

func (r *repo) ClusterOperationCreateOrUpdate(ctx context.Context, tenantId, envId uuid.UUID, masterVersion string, op *containerpb.Operation) (gensql.ClusterUpgrade, error) {
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

	return r.querier.ClusterOperationCreateOrUpdate(ctx, gensql.ClusterOperationCreateOrUpdateParams{
		Operationid:         op.Name,
		Status:              op.Status.String(),
		Tenantid:            tenantId,
		Envid:               envId,
		Type:                op.OperationType.String(),
		Masterversion:       pgtype.Text{String: masterVersion, Valid: true},
		Nodestotal:          int32(nodes_total),
		Nodesfailed:         int32(nodes_failed),
		Nodescompleted:      int32(nodes_complete),
		Nodesdone:           int32(nodes_done),
		Nodepdbdelayseconds: int32(node_pdb_delay_seconds),
	})
}
