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
-- For each feature, count how many environments have a failed or
-- pending status. Merges deploy_instructions (naisd responded) with
-- deployment_statuses (naisd may be unreachable) so that pending
-- deployments where no deploy instruction was ever created are included.
WITH di_latest AS (
	SELECT DISTINCT ON (feature_name,
		environment_id)
		feature_name,
		environment_id,
		status
	FROM
		deploy_instructions
	ORDER BY
		feature_name,
		environment_id,
		created DESC
),
ds_latest AS (
	SELECT
		d.feature_name,
		ds.environment_id,
		ds.status
	FROM
		deployment_statuses ds
		JOIN deployments d ON d.id = ds.deployment_id
	WHERE
		NOT EXISTS (
			SELECT
				1
			FROM
				di_latest di
			WHERE
				di.feature_name = d.feature_name
				AND di.environment_id = ds.environment_id)
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
	COUNT(*) FILTER (WHERE status IN ('failed', 'FAILED')) > 0
	OR COUNT(*) FILTER (WHERE status IN ('pending', 'created', 'PENDING', 'CREATED')) > 0;

