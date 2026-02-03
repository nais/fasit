-- name: InsertEnvironmentFeature :exec
INSERT INTO environment_features(
	environment_id,
	feature_name,
	feature_version,
	deployment_id)
VALUES (
	@environment_id,
	@feature_name,
	@feature_version,
	@deployment_id)
ON CONFLICT (
	environment_id,
	feature_name)
	DO UPDATE SET
		feature_version = EXCLUDED.feature_version,
		deployment_id = EXCLUDED.deployment_id;

-- name: GetEnvironmentFeature :one
SELECT
	sqlc.embed(fd),
	ef.deployment_id
FROM
	environment_features ef
	JOIN feature_data fd ON fd.name = ef.feature_name
		AND fd.version = ef.feature_version
WHERE
	environment_id = @environment_id
	AND feature_name = @feature_name;

-- name: ListEnvironmentFeatures :many
SELECT
	sqlc.embed(fd),
	d.created
FROM
	environment_features ef
	JOIN deployments d ON d.id = ef.deployment_id
	JOIN feature_data fd ON fd.name = ef.feature_name
		AND fd.version = ef.feature_version
WHERE
	environment_id = @environment_id
ORDER BY
	fd.name ASC;

