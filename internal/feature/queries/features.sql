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

-- name: FeatureStateCreateOrUpdate :one
INSERT INTO feature_states(
	environment_id,
	feature,
	enabled,
	enabled_at)
VALUES (
	@environment_id,
	@feature,
	@enabled,
	@enabledAt)
ON CONFLICT (
	environment_id,
	feature)
	DO UPDATE SET
		enabled = EXCLUDED.enabled,
		enabled_at = EXCLUDED.enabled_at
	RETURNING
		*;

-- name: FeatureStatesGet :many
WITH env AS (
	SELECT
		kind
	FROM
		environments
	WHERE
		id = @environment_id
),
filtered AS (
	SELECT DISTINCT ON (name)
		name,
		version
	FROM
		features
		JOIN env ON 1 = 1
	ORDER BY
		name,
		last_modified
)
SELECT
	@environment_id::UUID AS environment_id,
	f.name,
	COALESCE(fs.enabled, FALSE) AS enabled,
	fs.created,
	fs.last_modified,
	fs.enabled_at
FROM
	filtered f
	JOIN feature_data fd ON fd.name = f.name
		AND fd.version = f.version
	LEFT JOIN feature_states fs ON fs.feature = f.name
		AND fs.environment_id = @environment_id
WHERE (
	SELECT
		kind
	FROM
		env) = ANY (fd.kinds)
ORDER BY
	f.name ASC;

-- name: FeatureStateGet :one
SELECT
	*
FROM
	feature_states
WHERE
	feature = @feature
	AND environment_id = @environment_id;

