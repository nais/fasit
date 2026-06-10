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
	fd.name = @feature_name::TEXT
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
	fd.name = @feature_name::TEXT
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
	@feature_name::TEXT,
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
	id = @id::UUID;

-- name: GetFeatureAssignment :one
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id::UUID;

-- name: ListFeatureAssignmentsForEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd),
(df.feature IS NOT NULL)::BOOL AS disabled
FROM
	feature_assignments d
	JOIN environments e ON e.id = @environment_id::UUID
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

-- name: DeactivateFeatureAssignmentsByFeatureAndTarget :exec
UPDATE
	feature_assignments
SET
	active = FALSE
WHERE
	feature_name = @feature_name::TEXT
	AND target = @target
	AND active = TRUE;

-- name: ListFeatureAssignmentsForFeatureInEnvironment :many
SELECT DISTINCT ON (d.feature_name, d.target)
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN environments e ON e.id = @environment_id::UUID
		AND e.labels @> d.target
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.feature_name = @feature_name::TEXT
	AND d.active = TRUE
ORDER BY
	d.feature_name,
	d.target,
	d.created DESC;

-- name: DeactivateActiveFeatureAssignmentForTarget :exec
UPDATE
	feature_assignments
SET
	active = FALSE
WHERE
	feature_name = @feature_name::TEXT
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
			feature_name = @feature_name::TEXT
			AND active = TRUE) AS has_active;

