-- name: DeployInstructionsByID :one
SELECT
	*
FROM
	deploy_instructions
WHERE
	id = @id;

-- name: DeployInstructionsUpdateStatus :exec
UPDATE
	deploy_instructions
SET
	status = @status
WHERE
	id = @id;

-- name: DeployInstructionsLatestForFeature :one
SELECT
	*
FROM
	deploy_instructions
WHERE
	feature_name = @feature_name
	AND environment_id = @environment_id
ORDER BY
	created DESC
LIMIT 1;

-- name: DeployInstructionsLatestDeployedForFeature :one
SELECT
	*
FROM
	deploy_instructions
WHERE
	feature_name = @feature_name
	AND environment_id = @environment_id
	AND status = 'deployed'
ORDER BY
	last_modified DESC
LIMIT 1;

-- name: DeployInstructionsForFeature :many
SELECT
	*
FROM
	deploy_instructions
WHERE
	feature_name = @feature_name
	AND environment_id = @environment_id
ORDER BY
	created DESC
LIMIT 10 OFFSET sqlc.arg('offset');

-- name: DeployInstructionsPrevious :one
WITH CURRENT AS (
	SELECT
		di.*
	FROM
		deploy_instructions di
	WHERE
		di.id = @id
)
SELECT
	*
FROM
	deploy_instructions
WHERE
	feature_name =(
		SELECT
			feature_name
		FROM
			CURRENT)
	AND environment_id =(
		SELECT
			environment_id
		FROM
			CURRENT)
	AND created <(
		SELECT
			created
		FROM
			CURRENT)
ORDER BY
	created DESC
LIMIT 1;

-- name: NamesFromDeployInstruction :one
SELECT
	environments.name AS environment_name,
	tenants.name AS tenant_name
FROM
	deploy_instructions
	JOIN environments ON deploy_instructions.environment_id = environments.id
	JOIN tenants ON environments.tenant_id = tenants.id
WHERE
	deploy_instructions.id = @id;

-- name: DeployInstructionStatusCounts :many
-- For each feature, count failed/pending statuses.
-- For deployment-based features: uses deployment_statuses (covers
-- both naisd-responded and naisd-unreachable cases).
-- For rollout-only features: uses deploy_instructions.
WITH has_deployment AS (
	SELECT DISTINCT
		feature_name
	FROM
		deployments
),
di_latest AS (
	SELECT DISTINCT ON (feature_name,
		environment_id)
		feature_name,
		status
	FROM
		deploy_instructions
	WHERE
		NOT EXISTS (
			SELECT
				1
			FROM
				has_deployment hd
			WHERE
				hd.feature_name = deploy_instructions.feature_name)
		ORDER BY
			feature_name,
			environment_id,
			created DESC
),
deployment_status_latest AS (
	SELECT DISTINCT ON (deployment_id)
		deployment_id,
		status
	FROM
		deployment_statuses
	ORDER BY
		deployment_id,
		created DESC
),
ds_latest AS (
	SELECT DISTINCT ON (d.feature_name,
		d.environment_id)
		d.feature_name,
		dsl.status
	FROM
		deployments d
		JOIN deployment_status_latest dsl ON dsl.deployment_id = d.id
	ORDER BY
		d.feature_name,
		d.environment_id,
		d.created DESC
),
combined AS (
	SELECT
		feature_name,
		status
	FROM
		di_latest
UNION ALL
SELECT
	feature_name,
	status
FROM
	ds_latest
)
SELECT
	feature_name,
	COUNT(*) FILTER (WHERE status = 'failed') AS failed_count,
	COUNT(*) FILTER (WHERE status IN ('pending', 'created')) AS pending_count
FROM
	combined
GROUP BY
	feature_name
HAVING
	COUNT(*) FILTER (WHERE status = 'failed') > 0
	OR COUNT(*) FILTER (WHERE status IN ('pending', 'created')) > 0;

