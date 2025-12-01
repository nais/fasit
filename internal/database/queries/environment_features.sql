-- name: InsertEnvironmentFeature :exec
INSERT INTO
	environment_features (
		environment_id,
		feature_name,
		feature_version,
		deployment_id
	)
VALUES
	(
		@environment_id,
		@feature_name,
		@feature_version,
		@deployment_id
	)
ON CONFLICT (environment_id, feature_name) DO UPDATE
SET
	feature_version = EXCLUDED.feature_version,
	deployment_id = EXCLUDED.deployment_id
;

-- name: GetEnvironmentFeature :one
SELECT
	fd.name,
	fd.version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds::TEXT[] AS kinds,
	fd.dependencies,
	fd.values,
	fd.default_values,
	fd.timeout,
	ef.deployment_id
FROM
	environment_features ef
	JOIN feature_data fd ON fd.name = ef.feature_name
	AND fd.version = ef.feature_version
WHERE
	environment_id = @environment_id
	AND feature_name = @feature_name
;
