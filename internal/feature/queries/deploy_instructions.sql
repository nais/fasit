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

-- name: ListLatestDeployInstructionsForEnvironment :many
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
	environment_id = @environment_id::UUID
ORDER BY
	feature_name;

-- name: ListLatestDeployedForEnvironment :many
SELECT DISTINCT ON (feature_name)
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
	environment_id = @environment_id::UUID
	AND status = 'deployed'
ORDER BY
	feature_name,
	created DESC;

-- name: ListLatestDeployInstructionsForFeature :many
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
ORDER BY
	environment_id;

-- name: ListLatestDeployedForFeature :many
SELECT DISTINCT ON (environment_id)
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
	AND status = 'deployed'
ORDER BY
	environment_id,
	created DESC;

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

