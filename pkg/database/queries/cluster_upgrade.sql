-- name: ClusterOperationCreateOrUpdate :one
INSERT INTO cluster_upgrade
("operation_id", "tenant_id", "environment_id", "status", "type", "nodes_total", "nodes_failed","nodes_completed", "nodes_done", "node_pdb_delay_seconds")
VALUES
(@operationId, @tenantId, @envID, @status, @type, @nodesTotal, @nodesFailed, @nodesCompleted, @nodesDone, @nodePdbDelaySeconds)
ON CONFLICT ("operation_id", "tenant_id", "environment_id") DO
UPDATE SET
    "status" = EXCLUDED.status,
    "nodes_total" = EXCLUDED.nodes_total,
    "nodes_failed" = EXCLUDED.nodes_failed,
    "nodes_completed" = EXCLUDED.nodes_completed,
    "nodes_done" = EXCLUDED.nodes_done,
    "node_pdb_delay_seconds" = EXCLUDED.node_pdb_delay_seconds
RETURNING *;

-- name: ClusterOperationsGet :many
SELECT * FROM cluster_upgrade WHERE "tenant_id" = @tenantId AND "environment_id" = @envID AND "status" = @status
ORDER BY "start_time" DESC;
