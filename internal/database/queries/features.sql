-- name: FeatureByName :one
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
	features.created,
	features.last_modified
FROM
	features
	JOIN feature_data fd ON features.name = fd.name
	AND features.version = fd.version
WHERE
	fd.name = @name
;

-- name: Features :many
WITH
	combined AS (
		SELECT
			NULL AS id,
			name,
			version,
			created,
			last_modified
		FROM
			features
		UNION
		(
			SELECT DISTINCT
				ON (feature_name) id,
				feature_name AS name,
				version,
				MAKE_TIMESTAMPTZ(1969, 4, 20, 0, 0, 0) AS created,
				MAKE_TIMESTAMPTZ(1969, 4, 20, 0, 0, 0) AS last_modified
			FROM
				rollouts
			WHERE
				status = 'pending'
			ORDER BY
				feature_name,
				"version" DESC
		)
	),
	filtered AS (
		SELECT DISTINCT
			ON (name) name AS name,
			version,
			created,
			last_modified
		FROM
			combined
		ORDER BY
			-- order by id to ensure rollout has precedence over feature
			name,
			id
	)
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
	combined.created,
	combined.last_modified
FROM
	combined
	JOIN feature_data fd ON combined.name = fd.name
	AND combined.version = fd.version
ORDER BY
	combined.name
;

-- name: FeatureGetForEnv :many
SELECT
	fd.*,
	features.created,
	features.last_modified
FROM
	features
	JOIN feature_data fd ON features.name = fd.name
	AND features.version = fd.version
WHERE
	@environment_kind::TEXT = ANY (kinds::TEXT[])
ORDER BY
	features.name
;

-- name: FeaturesForKind :many
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
	features.created,
	features.last_modified
FROM
	features
	JOIN feature_data fd ON features.name = fd.name
	AND features.version = fd.version
WHERE
	@environment_kind::TEXT = ANY (kinds::TEXT[])
ORDER BY
	features.name
;

-- name: FeatureVersionUpdate :exec
INSERT INTO
	features (name, version)
VALUES
	(@name, @version)
ON CONFLICT (name) DO UPDATE
SET
	version = EXCLUDED.version
;
