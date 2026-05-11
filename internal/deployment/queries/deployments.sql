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
	description)
VALUES (
	@feature_name,
	@version,
	@target,
	@gh_ref,
	@description)
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
WHERE
	NOT EXISTS (
		SELECT
			1
		FROM
			disabled_features df
		WHERE
			df.environment_id = e.id
			AND df.feature = fd.name)
ORDER BY
	d.feature_name,
	d.target,
	d.created DESC;

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
WITH statuses AS (
	SELECT
		deployment_id,
		environment_id,
		status,
		message,
		last_modified,
		created
	FROM
		deployment_statuses
	WHERE
		deployment_id = @deployment_id
),
disabled AS (
	SELECT
		d.id AS deployment_id,
		e.id AS environment_id,
		'DISABLED' AS status,
		'feature is disabled in this environment' AS message,
		df.disabled_at AS last_modified,
		df.disabled_at AS created
	FROM
		environments e
		JOIN disabled_features df ON df.environment_id = e.id
		JOIN deployments d ON df.feature = d.feature_name
	WHERE
		e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
		AND d.id = @deployment_id
),
computed AS (
	SELECT
		*
	FROM
		statuses
	UNION
	SELECT
		*
	FROM
		disabled
)
SELECT
	*
FROM
	computed
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

-- name: DeleteDeploymentsByFeatureAndTarget :exec
DELETE FROM deployments
WHERE feature_name = @feature_name
	AND target = @target;

-- name: GetDeploymentStatus :one
SELECT
	*
FROM
	deployment_statuses ds
WHERE
	ds.deployment_id = @deployment_id
	AND ds.environment_id = @environment_id;

