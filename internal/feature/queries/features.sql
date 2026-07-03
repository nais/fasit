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
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.feature_name = @feature_name
	AND d.active = TRUE
ORDER BY
	d.created DESC
LIMIT 1;

-- name: FeatureDataByVersion :one
SELECT
	sqlc.embed(fd)
FROM
	feature_data fd
WHERE
	fd.name = @feature_name
	AND fd.version = @version;

-- name: FeatureVersionRows :many
SELECT
	fd.name,
	fd.version,
	fd.description,
	fd.source,
	MAX(fa.created) AS last_updated
FROM
	feature_data fd
	LEFT JOIN feature_assignments fa ON fa.feature_name = fd.name
		AND fa.version = fd.version
WHERE
	fd.name = @feature_name
GROUP BY
	fd.name,
	fd.version,
	fd.description,
	fd.source
ORDER BY
	last_updated DESC NULLS LAST,
	fd.version DESC;

-- name: FeatureNames :many
SELECT DISTINCT
	feature_name
FROM
	feature_assignments
WHERE
	active = TRUE
ORDER BY
	feature_name;

-- name: ListActiveFeatures :many
SELECT DISTINCT ON (d.feature_name)
	fd.name,
	fd.description,
	fd.source
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.active = TRUE
ORDER BY
	d.feature_name,
	d.created DESC;

