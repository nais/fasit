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
WITH has_deployment AS (
	SELECT DISTINCT
		feature_name
	FROM
		deployments
),
di_latest AS (
	SELECT
		di.feature_name,
		di.status
	FROM
		deploy_instructions di
		INNER JOIN (
			SELECT
				feature_name,
				environment_id,
				MAX(created) AS max_created
			FROM
				deploy_instructions
			GROUP BY
				feature_name,
				environment_id) latest ON di.feature_name = latest.feature_name
			AND di.environment_id = latest.environment_id
			AND di.created = latest.max_created
		WHERE
			NOT EXISTS (
				SELECT
					1
				FROM
					has_deployment hd
				WHERE
					hd.feature_name = di.feature_name)
),
ds_latest AS (
	SELECT
		d.feature_name,
		COALESCE(di.status, ds.status) AS status
	FROM
		deployment_statuses ds
		JOIN deployments d ON d.id = ds.deployment_id
		LEFT JOIN (
			SELECT
				feature_name,
				environment_id,
				status,
				ROW_NUMBER() OVER (PARTITION BY feature_name,
					environment_id ORDER BY created DESC) AS rn
			FROM
				deploy_instructions) di ON di.feature_name = d.feature_name
				AND di.environment_id = ds.environment_id
				AND di.rn = 1
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
	OR COUNT(*) FILTER (WHERE status IN ('pending', 'created')) > 0
ORDER BY
	feature_name;

