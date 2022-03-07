-- name: StatusCreateOrUpdate :exec
INSERT INTO status (environment_id, feature, version, status, config_hash)
VALUES (@environment_id, @feature, @version, @status, @config_hash)
ON CONFLICT (environment_id, feature)
DO
UPDATE SET version=EXCLUDED.version, status=EXCLUDED.status, config_hash=EXCLUDED.config_hash;

-- name: StatusForEnvironment :many
SELECT *
FROM status
WHERE environment_id = @environment_id;
