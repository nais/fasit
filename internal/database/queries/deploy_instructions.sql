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
-- For each feature, count how many environments have a latest deploy
-- instruction in failed or pending/created state. Used by the features
-- sidebar to show badge counts without per-feature queries.
WITH latest AS (
	SELECT DISTINCT ON (feature_name,
		environment_id)
		feature_name,
		status
	FROM
		deploy_instructions
	ORDER BY
		feature_name,
		environment_id,
		created DESC
)
SELECT
	feature_name,
	COUNT(*) FILTER (WHERE status = 'failed') AS failed_count,
	COUNT(*) FILTER (WHERE status IN ('pending', 'created')) AS pending_count
FROM
	latest
GROUP BY
	feature_name
HAVING
	COUNT(*) FILTER (WHERE status = 'failed') > 0
	OR COUNT(*) FILTER (WHERE status IN ('pending', 'created')) > 0;

