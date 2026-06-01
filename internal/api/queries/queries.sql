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

