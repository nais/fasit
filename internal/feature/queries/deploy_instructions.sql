-- Deploy state is derived from deploy_log. Per-environment current state comes
-- from the deploy_status view (latest row per environment x feature). Deploy
-- history (one entry per deploy) is grouped by diid in Go from ListDeployLog.
-- name: GetLatestDeployInstruction :one
SELECT
	diid AS id,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash,
	created
FROM
	deploy_status
WHERE
	feature_name = @feature_name::TEXT
	AND environment_id = @environment_id::UUID;

-- name: GetLatestDeployedDeployInstruction :one
SELECT
	diid AS id,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash,
	created
FROM
	deploy_log
WHERE
	feature_name = @feature_name::TEXT
	AND environment_id = @environment_id::UUID
	AND status = 'deployed'
ORDER BY
	created DESC
LIMIT 1;

-- name: ListDeployLog :many
SELECT
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash,
	created
FROM
	deploy_log
WHERE
	feature_name = @feature_name::TEXT
	AND environment_id = @environment_id::UUID
ORDER BY
	created ASC;

