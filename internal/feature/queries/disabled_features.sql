-- name: SetDisabledFeature :exec
INSERT INTO disabled_features(
	environment_id,
	feature)
VALUES (
	@environment_id,
	@feature)
ON CONFLICT (
	environment_id,
	feature)
	DO NOTHING;

-- name: DeleteDisabledFeature :exec
DELETE FROM disabled_features
WHERE environment_id = @environment_id
	AND feature = @feature;

-- name: GetDisabledFeature :one
SELECT
	*
FROM
	disabled_features
WHERE
	environment_id = @environment_id
	AND feature = @feature;

-- name: ListDisabledFeaturesByEnvironment :many
SELECT
	*
FROM
	disabled_features
WHERE
	environment_id = @environment_id
ORDER BY
	feature;

-- name: ListDisabledFeatureEnvironments :many
SELECT
	environment_id,
	disabled_at
FROM
	disabled_features
WHERE
	feature = @feature
ORDER BY
	environment_id;

