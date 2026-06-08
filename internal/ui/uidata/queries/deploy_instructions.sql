-- Deploy history for the UI, derived from deploy_log. Publish row (earliest per
-- diid) carries version/hash/values; current status is the latest row per diid.
-- name: ListDeployInstructions :many
WITH publish AS (
	SELECT DISTINCT ON (diid)
		diid,
		environment_id,
		feature_assignment_id,
		feature_name,
		feature_version,
		hash,
		"values",
		created
	FROM
		deploy_log
	WHERE
		feature_assignment_id = @feature_assignment_id::UUID
	ORDER BY
		diid,
		created ASC
),
current_status AS (
	SELECT DISTINCT ON (diid)
		diid,
		status,
		created AS last_modified
	FROM
		deploy_log
	WHERE
		feature_assignment_id = @feature_assignment_id::UUID
	ORDER BY
		diid,
		created DESC
)
SELECT
	p.diid AS id,
	p.environment_id,
	p.feature_assignment_id,
	p.feature_name,
	p.feature_version,
	cs.status,
	p.hash,
	p.created,
	cs.last_modified,
	p."values",
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	publish p
	JOIN current_status cs ON cs.diid = p.diid
	JOIN environments e ON e.id = p.environment_id
	JOIN tenants t ON t.id = e.tenant_id
ORDER BY
	p.created DESC;

-- name: GetDeployInstructionByFeatureAssignmentAndEnvironmentID :one
WITH publish AS (
	SELECT DISTINCT ON (diid)
		diid,
		environment_id,
		feature_assignment_id,
		feature_name,
		feature_version,
		hash,
		"values",
		created
	FROM
		deploy_log
	WHERE
		feature_assignment_id = @feature_assignment_id::UUID
		AND environment_id = @environment_id::UUID
	ORDER BY
		diid,
		created ASC
),
current_status AS (
	SELECT DISTINCT ON (diid)
		diid,
		status,
		created AS last_modified
	FROM
		deploy_log
	WHERE
		feature_assignment_id = @feature_assignment_id::UUID
		AND environment_id = @environment_id::UUID
	ORDER BY
		diid,
		created DESC
)
SELECT
	p.diid AS id,
	p.environment_id,
	p.feature_assignment_id,
	p.feature_name,
	p.feature_version,
	cs.status,
	p.hash,
	p.created,
	cs.last_modified,
	p."values",
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	publish p
	JOIN current_status cs ON cs.diid = p.diid
	JOIN environments e ON e.id = p.environment_id
	JOIN tenants t ON t.id = e.tenant_id
ORDER BY
	p.created DESC
LIMIT 1;

