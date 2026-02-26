-- name: ListDeployments :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN (
		SELECT
			feature_name,
			target,
			MAX(created) AS max_created
		FROM
			deployments
		GROUP BY
			feature_name,
			target) latest ON d.feature_name = latest.feature_name
	AND d.target = latest.target
	AND d.created = latest.max_created
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	ORDER BY
		d.created DESC;

-- name: ListDeploymentsByFeature :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	fd.name = @feature_name
ORDER BY
	d.created DESC;

-- name: CreateDeployment :one
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

-- name: DeleteDeployment :exec
DELETE FROM deployments
WHERE id = @id;

-- name: GetDeployment :one
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id;

-- name: ListDeploymentsToReconcile :many
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

-- name: SetDeploymentStatus :exec
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

-- name: ListDeploymentStatuses :many
SELECT
	*
FROM
	deployment_statuses
WHERE
	deployment_id = @deployment_id
ORDER BY
	last_modified DESC,
	environment_id ASC;

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

