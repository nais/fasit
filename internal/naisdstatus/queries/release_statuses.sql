-- name: ListReleaseStatuses :many
SELECT
	*
FROM
	release_statuses
WHERE
	environment_id = @environment_id
ORDER BY
	feature ASC;

-- name: GetReleaseStatus :one
SELECT
	*
FROM
	release_statuses
WHERE
	environment_id = @environment_id
	AND feature = @feature;

-- name: ListReleaseStatusesForFeature :many
SELECT
	*
FROM
	release_statuses
WHERE
	feature = @feature
ORDER BY
	environment_id;

