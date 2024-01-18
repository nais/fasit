-- name: ClusterVersionCreate :one
INSERT INTO cluster_version
("tenant_id", "environment_id", "version")
VALUES
(@tenantId, @envID, @version)
RETURNING *;

-- name: ClusterVersionGet :one
SELECT * FROM cluster_version WHERE "tenant_id" = @tenantId AND "environment_id" = @envID AND "version" = @version;

-- name: ClusterVersionUpdateStatus :exec
UPDATE cluster_version SET "status" = @status WHERE "tenant_id" = @tenantId AND "environment_id" = @envID AND "version" = @version;
