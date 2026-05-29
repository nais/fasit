-- name: ListAllDeployments :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN ( SELECT DISTINCT ON (feature_name, target)
			id
		FROM
			deployments
		ORDER BY
			feature_name,
			target,
			active DESC,
			created DESC) best ON d.id = best.id
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
	AND d.active = TRUE
ORDER BY
	d.created DESC;

-- name: ListAllDeploymentsByFeature :many
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
	d.active DESC,
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

-- name: DeactivateDeployment :exec
UPDATE
	deployments
SET
	active = FALSE
WHERE
	id = @id;

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

-- name: ListDeploymentsForEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd),
(df.feature IS NOT NULL)::BOOL AS disabled
FROM
	deployments d
	JOIN environments e ON e.id = @environment_id
		AND e.labels @> d.target
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	LEFT JOIN disabled_features df ON df.environment_id = e.id
		AND df.feature = d.feature_name
WHERE
	d.active = TRUE
ORDER BY
	d.feature_name,
	d.target,
	d.created DESC;

-- name: ListDeployedFeaturesInEnvironment :many
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

-- name: DeactivateDeploymentsByFeatureAndTarget :exec
UPDATE
	deployments
SET
	active = FALSE
WHERE
	feature_name = @feature_name
	AND target = @target
	AND active = TRUE;

-- name: GetDeploymentStatus :one
SELECT
	*
FROM
	deployment_statuses ds
WHERE
	ds.deployment_id = @deployment_id
	AND ds.environment_id = @environment_id;

-- name: ListDeploymentsForFeatureInEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN environments e ON e.id = @environment_id
		AND e.labels @> d.target
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.feature_name = @feature_name
	AND d.active = TRUE
ORDER BY
	d.feature_name,
	d.target,
	d.created DESC;

-- name: ListRecentDeployments :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	ORDER BY
		d.created DESC
	LIMIT 50;

-- name: DeactivateActiveDeploymentForTarget :exec
UPDATE
	deployments
SET
	active = FALSE
WHERE
	feature_name = @feature_name
	AND target = @target
	AND active = TRUE;

