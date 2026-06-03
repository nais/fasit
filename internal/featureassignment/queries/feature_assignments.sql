-- name: ListAllFeatureAssignments :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN ( SELECT DISTINCT ON (feature_name, target)
			id
		FROM
			feature_assignments
		ORDER BY
			feature_name,
			target,
			active DESC,
			created DESC) best ON d.id = best.id
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	ORDER BY
		d.created DESC;

-- name: ListFeatureAssignmentsByFeature :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	fd.name = @feature_name
	AND d.active = TRUE
ORDER BY
	d.created DESC;

-- name: ListAllFeatureAssignmentsByFeature :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	fd.name = @feature_name
ORDER BY
	d.active DESC,
	d.created DESC;

-- name: CreateFeatureAssignment :one
INSERT INTO feature_assignments(
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

-- name: DeactivateFeatureAssignment :exec
UPDATE
	feature_assignments
SET
	active = FALSE
WHERE
	id = @id;

-- name: GetFeatureAssignment :one
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id;

-- name: ListFeatureAssignmentsForEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd),
(df.feature IS NOT NULL)::BOOL AS disabled
FROM
	feature_assignments d
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

-- name: SetReconcileStatus :exec
INSERT INTO feature_reconcile_statuses(
	feature_assignment_id,
	environment_id,
	status,
	message)
VALUES (
	@feature_assignment_id,
	@environment_id,
	@status,
	@message)
ON CONFLICT (
	feature_assignment_id,
	environment_id)
	DO UPDATE SET
		status = EXCLUDED.status,
		message = EXCLUDED.message;

-- name: ListReconcileStatuses :many
WITH statuses AS (
	SELECT
		feature_assignment_id,
		environment_id,
		status,
		message,
		last_modified,
		created
	FROM
		feature_reconcile_statuses
	WHERE
		feature_assignment_id = @feature_assignment_id
),
disabled AS (
	SELECT
		d.id AS feature_assignment_id,
		e.id AS environment_id,
		'DISABLED' AS status,
		'feature is disabled in this environment' AS message,
		df.disabled_at AS last_modified,
		df.disabled_at AS created
	FROM
		environments e
		JOIN disabled_features df ON df.environment_id = e.id
		JOIN feature_assignments d ON df.feature = d.feature_name
	WHERE
		e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
		AND d.id = @feature_assignment_id
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

-- name: LatestReconcileStatusForEnvironment :one
SELECT
	status
FROM
	feature_reconcile_statuses
WHERE
	feature_assignment_id = @feature_assignment_id
	AND environment_id = @environment_id
ORDER BY
	last_modified DESC
LIMIT 1;

-- name: DeactivateFeatureAssignmentsByFeatureAndTarget :exec
UPDATE
	feature_assignments
SET
	active = FALSE
WHERE
	feature_name = @feature_name
	AND target = @target
	AND active = TRUE;

-- name: GetReconcileStatus :one
SELECT
	*
FROM
	feature_reconcile_statuses ds
WHERE
	ds.feature_assignment_id = @feature_assignment_id
	AND ds.environment_id = @environment_id;

-- name: ListFeatureAssignmentsForFeatureInEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
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

-- name: ListRecentFeatureAssignments :many
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
	ORDER BY
		d.created DESC
	LIMIT 50;

-- name: DeactivateActiveFeatureAssignmentForTarget :exec
UPDATE
	feature_assignments
SET
	active = FALSE
WHERE
	feature_name = @feature_name
	AND target = @target
	AND active = TRUE;

-- name: HasActiveAssignments :one
SELECT
	EXISTS (
		SELECT
			1
		FROM
			feature_assignments
		WHERE
			feature_name = @feature_name
			AND active = TRUE) AS has_active;

