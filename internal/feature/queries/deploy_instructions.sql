-- Deploy history is derived from deploy_log. Each deploy is identified by its
-- diid: the publish row (earliest row for the diid) carries version/hash/values,
-- and the current status is the latest row for that diid. A "deploy instruction"
-- in these queries is the publish row joined with its current status.
-- name: GetPreviousDeployInstruction :one
WITH cur AS (
	SELECT
		environment_id,
		feature_name,
		created
	FROM
		deploy_log
	WHERE
		diid = @id::UUID
	ORDER BY
		created ASC
	LIMIT 1
),
publish AS (
	SELECT DISTINCT ON (dl.diid)
		dl.diid,
		dl.environment_id,
		dl.feature_assignment_id,
		dl.feature_name,
		dl.feature_version,
		dl.hash,
		dl."values",
		dl.created
	FROM
		deploy_log dl,
		cur
	WHERE
		dl.environment_id = cur.environment_id
		AND dl.feature_name = cur.feature_name
		AND dl.created < cur.created
	ORDER BY
		dl.diid,
		dl.created ASC
),
current_status AS (
	SELECT DISTINCT ON (diid)
		diid,
		status,
		created AS last_modified
	FROM
		deploy_log
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
	p."values"
FROM
	publish p
	JOIN current_status cs ON cs.diid = p.diid
ORDER BY
	p.created DESC
LIMIT 1;

-- name: GetLatestDeployInstruction :one
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
		feature_name = @feature_name::TEXT
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
		feature_name = @feature_name::TEXT
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
	p."values"
FROM
	publish p
	JOIN current_status cs ON cs.diid = p.diid
ORDER BY
	p.created DESC
LIMIT 1;

-- name: GetLatestDeployedDeployInstruction :one
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
		feature_name = @feature_name::TEXT
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
		feature_name = @feature_name::TEXT
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
	p."values"
FROM
	publish p
	JOIN current_status cs ON cs.diid = p.diid
WHERE
	cs.status = 'deployed'
ORDER BY
	cs.last_modified DESC
LIMIT 1;

-- name: ListRecentDeployInstructions :many
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
		feature_name = @feature_name::TEXT
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
		feature_name = @feature_name::TEXT
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
	p."values"
FROM
	publish p
	JOIN current_status cs ON cs.diid = p.diid
ORDER BY
	p.created DESC
LIMIT sqlc.arg('limit');

