-- name: DeploymentsGet :many
SELECT
	sqlc.embed(d),
	fd.name,
	fd.version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds::TEXT[] AS kinds,
	fd.dependencies,
	fd.values,
	fd.default_values,
	fd.timeout
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	ORDER BY
		d.created DESC;

-- name: DeploymentsGetByFeature :many
SELECT
	sqlc.embed(d),
	fd.name,
	fd.version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds::TEXT[] AS kinds,
	fd.dependencies,
	fd.values,
	fd.default_values,
	fd.timeout
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	fd.name = @feature_name
ORDER BY
	d.created DESC;

-- name: DeploymentCreate :one
INSERT INTO deployments(
	feature_name,
	version,
	target,
	gh_ref,
	description)
VALUES (
	@feature_name,
	@version,
	@target,
	@gh_ref,
	@description)
RETURNING
	*;

-- name: DeploymentDelete :exec
DELETE FROM deployments
WHERE id = @id;

-- name: DeploymentGet :one
SELECT
	sqlc.embed(d),
	fd.name,
	fd.version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds::TEXT[] AS kinds,
	fd.dependencies,
	fd.values,
	fd.default_values,
	fd.timeout
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id;

-- name: FeatureDeploymentsForEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	fd.name,
	fd.version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds::TEXT[] AS kinds,
	fd.dependencies,
	fd.values,
	fd.default_values,
	fd.timeout,
	ds.status,
	ds.message AS status_message,
	ds.last_modified AS status_last_modified,
	ds.created AS status_created
FROM
	deployments d
	JOIN environments e ON e.id = @environment_id
		AND e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	LEFT JOIN deployment_statuses ds ON ds.deployment_id = d.id
		AND ds.environment_id = @environment_id
	ORDER BY
		d.feature_name,
		d.target,
		d.created DESC;

-- name: FeatureEnabled :one
SELECT
	NOT EXISTS (
		SELECT
			*
		FROM
			feature_states fs
		WHERE
			fs.feature = @feature_name
			AND fs.environment_id = @environment_id
			AND fs.enabled = FALSE);

-- name: DeployInstructionsGetDeployedFeatures :many
SELECT DISTINCT ON (feature_name)
	feature_name
FROM
	deploy_instructions
WHERE
	feature_name = ANY (@feature_names::TEXT[])
	AND status = 'deployed'
	AND environment_id = @environment_id
ORDER BY
	feature_name;

-- name: DeploymentStatusCreateOrUpdate :exec
INSERT INTO deployment_statuses(
	deployment_id,
	environment_id,
	status,
	message)
VALUES (
	@deployment_id,
	@environment_id,
	@status,
	@message)
ON CONFLICT (
	deployment_id,
	environment_id)
	DO UPDATE SET
		status = EXCLUDED.status,
		message = EXCLUDED.message;

-- name: DeploymentStatusGet :many
SELECT
	*
FROM
	deployment_statuses
WHERE
	deployment_id = @deployment_id
ORDER BY
	last_modified DESC,
	environment_id ASC;

