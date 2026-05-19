-- name: FeatureDataCreate :exec
INSERT INTO feature_data(
	name,
	version,
	chart,
	description,
	source,
	kinds,
	dependencies,
	"values",
	default_values,
	timeout,
	tpl_details)
VALUES (
	@feature_name,
	@version,
	@chart,
	@description,
	@source,
(
		@kinds::TEXT[]) ::environment_kind[],
	@dependencies,
	@values,
	@default_values,
	@timeout,
	@tpl_details);

-- name: LatestFeatureData :one
SELECT
	sqlc.embed(fd)
FROM
	deployments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.feature_name = @feature_name
ORDER BY
	d.created DESC
LIMIT 1;

-- name: FeatureNames :many
SELECT DISTINCT
	feature_name
FROM
	deployments
ORDER BY
	feature_name;

