-- name: ClusterUpgradesCreate :one
INSERT INTO cluster_upgrades
("tenant_id", "environment_id", "version")
VALUES
(@tenantId, @envID, @version)
RETURNING *;

-- name: ClusterUpgradesGet :many
SELECT * FROM cluster_upgrades
WHERE tenant_id = @tenantId
AND environment_id = @envID
AND status != 'DONE'
ORDER BY last_modified DESC;

-- name: ClusterUpgradesUpdateStatus :one
UPDATE cluster_upgrades
SET "status" = @status
WHERE "tenant_id" = @tenantId
AND "environment_id" = @envID
AND "version" = @version
RETURNING *;

-- name: ClusterUpgradesGetByID :one
SELECT * FROM cluster_upgrades
WHERE id = @id;
