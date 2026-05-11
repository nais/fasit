-- name: DisabledFeatureSet :exec
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

-- name: DisabledFeatureDelete :exec
DELETE FROM disabled_features
WHERE environment_id = @environment_id
	AND feature = @feature;

-- name: DisabledFeatureGet :one
SELECT
	*
FROM
	disabled_features
WHERE
	environment_id = @environment_id
	AND feature = @feature;

-- name: DisabledFeaturesByEnvironment :many
SELECT
	*
FROM
	disabled_features
WHERE
	environment_id = @environment_id
ORDER BY
	feature;

