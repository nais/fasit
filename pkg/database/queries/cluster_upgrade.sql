-- name: ClusterOperationCreateOrUpdate :one
INSERT INTO cluster_upgrade
("operation_id", "tenant_id", "environment_id", "status", "type", "master_version", "nodes_total", "nodes_failed","nodes_completed", "nodes_done", "node_pdb_delay_seconds")
VALUES
(@operationId, @tenantId, @envID, @status, @type, @masterVersion, @nodesTotal, @nodesFailed, @nodesCompleted, @nodesDone, @nodePdbDelaySeconds)
ON CONFLICT ("operation_id", "tenant_id", "environment_id") DO
UPDATE SET
    "status" = EXCLUDED.status,
    "master_version" = EXCLUDED.master_version,
    "nodes_total" = EXCLUDED.nodes_total,
    "nodes_failed" = EXCLUDED.nodes_failed,
    "nodes_completed" = EXCLUDED.nodes_completed,
    "nodes_done" = EXCLUDED.nodes_done,
    "node_pdb_delay_seconds" = EXCLUDED.node_pdb_delay_seconds
RETURNING *;
