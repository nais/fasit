-- name: ReleaseStatusCreateOrUpdate :one
INSERT INTO release_statuses
	(environment_id, feature, version, status, revision, last_deployed)
VALUES
	(@environment_id, @feature, @version, @status, @revision, @last_deployed)
ON CONFLICT (environment_id, feature) DO UPDATE
	SET
    version = EXCLUDED.version,
    status = EXCLUDED.status,
    revision = EXCLUDED.revision,
    last_deployed = EXCLUDED.last_deployed
RETURNING *;

-- name: ReleaseStatusesGet :many
SELECT * FROM release_statuses
WHERE environment_id = @environment_ID
ORDER BY feature ASC
;

-- name: ReleaseStatusDeleteByEnvironment :exec
DELETE FROM release_statuses
WHERE environment_id = @environment_id;
