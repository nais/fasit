-- name: HealthStatusCreateOrUpdate :one
INSERT INTO health_statuses
	(environment_id, reported_at)
VALUES
	(@environment_id, @reported_at)
ON CONFLICT (environment_id) DO UPDATE
	SET
    reported_at = EXCLUDED.reported_at
RETURNING *;

-- name: HealthStatusGet :one
SELECT * FROM health_statuses
WHERE environment_id = @environment_id;
