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

-- name: FeatureVersionUpdate :exec
INSERT INTO features(
	name,
	version)
VALUES (
	@name,
	@version)
ON CONFLICT (
	name)
	DO UPDATE SET
		version = EXCLUDED.version;

-- name: FeatureByName :one
SELECT
	sqlc.embed(fd),
	features.created,
	features.last_modified
FROM
	features
	JOIN feature_data fd ON features.name = fd.name
		AND features.version = fd.version
WHERE
	fd.name = @name;

-- name: Features :many
SELECT
	sqlc.embed(fd),
	features.created,
	features.last_modified
FROM
	features
	JOIN feature_data fd ON features.name = fd.name
		AND features.version = fd.version
	ORDER BY
		features.name;

