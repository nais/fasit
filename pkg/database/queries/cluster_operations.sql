-- name: ClusterOperationCreateOrUpdate :one
INSERT INTO cluster_operations
("id", "operation_name", "tenant_id", "environment_id", "upgrade_id", "status", "type", "detail", "nodes_total", "nodes_failed", "nodes_completed", "nodes_done", "node_pdb_delay_seconds")
VALUES
(@id, @operation_name, @tenant_id, @env_id, @upgrade_id, @status, @type, @detail, @nodes_total, @nodes_failed, @nodes_completed, @nodes_done, @node_pdb_delay_seconds)
ON CONFLICT ("id") DO
UPDATE SET
    "status" = EXCLUDED.status,
    "detail" = EXCLUDED.detail,
    "nodes_total" = EXCLUDED.nodes_total,
    "nodes_failed" = EXCLUDED.nodes_failed,
    "nodes_completed" = EXCLUDED.nodes_completed,
    "nodes_done" = EXCLUDED.nodes_done,
    "node_pdb_delay_seconds" = EXCLUDED.node_pdb_delay_seconds
RETURNING *;

-- name: ClusterOperationsGet :many
SELECT * FROM cluster_operations WHERE "tenant_id" = @tenantId AND "environment_id" = @envID AND "status" = @status
ORDER BY "start_time" DESC;

-- name: ClusterOperationGet :one
SELECT * FROM cluster_operations WHERE "tenant_id" = @tenantId AND "environment_id" = @envID AND "status" = @status
ORDER BY "start_time" DESC LIMIT 1;

-- name: ClusterOperationsGetByUpgradeID :one
SELECT * FROM cluster_operations WHERE "upgrade_id" = @upgrade_id ORDER BY "start_time" DESC LIMIT 1;

-- name: ClusterOperationsGetByID :one
SELECT * FROM cluster_operations
WHERE id = @id;
