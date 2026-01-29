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
	description,
	ci)
VALUES (
	@feature_name,
	@version,
	@target,
	@gh_ref,
	@description,
	@ci)
RETURNING
	*;

-- name: DeploymentDelete :exec
DELETE FROM deployments
WHERE id = @id;

-- name: DeploymentGet :one
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id;

-- name: DeploymentsForEnvironmentToReconcile :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN environments e ON e.id = @environment_id
		AND e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	LEFT JOIN feature_states fs ON fs.environment_id = e.id
		AND fs.feature = fd.name
WHERE
	COALESCE(fs.enabled, TRUE) = TRUE
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

-- name: GetCIEnvironmentsForTarget :many
SELECT DISTINCT
	sqlc.embed(e_ci),
	t.name AS tenant_name
FROM
	environments e_ci
	JOIN tenants t ON e_ci.tenant_id = t.id
WHERE
	e_ci.ci = TRUE
	AND EXISTS (
		SELECT
			1
		FROM
			environments e_non_ci
		WHERE
			e_non_ci.ci = FALSE
			AND e_non_ci.labels @> @target
			AND e_non_ci.kind = e_ci.kind)
ORDER BY
	e_ci.name ASC;

-- name: LatestStatusForDeploymentInEnvironment :one
SELECT
	status
FROM
	deployment_statuses
WHERE
	deployment_id = @deployment_id
	AND environment_id = @environment_id
ORDER BY
	last_modified DESC
LIMIT 1;

