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

