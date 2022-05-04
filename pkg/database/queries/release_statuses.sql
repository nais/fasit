-- name: ReleaseStatusCreateOrUpdate :one
INSERT INTO release_statuses
	(environment_id, feature, version, status, revision, last_deployed)
VALUES
	(@environment_id, @feature, @version, @status, @revision, @last_deployed)
ON CONFLICT (environment_id, feature, key) DO UPDATE
	SET
    version = EXCLUDED.version,
    status = EXCLUDED.status,
    revision = EXCLUDED.revision,
    last_deployed = EXCLUDED.last_deployed
RETURNING *;
