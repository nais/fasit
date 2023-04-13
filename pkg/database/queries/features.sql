-- name: FeatureByName :one
SELECT
  fd.name,
  fd.version,
  fd.chart,
  fd.description,
  fd.source,
  fd.kinds::text[] AS kinds,
  fd.dependencies,
  fd.values,
  fd.timeout,
  fd.default_values,
  features.created,
  features.last_modified
FROM features
JOIN feature_data fd ON features.name = fd.name AND features.version = fd.version
WHERE fd.name = @name
;

-- name: Features :many
SELECT
  fd.name,
  fd.version,
  fd.chart,
  fd.description,
  fd.source,
  fd.kinds::text[] AS kinds,
  fd.dependencies,
  fd.values,
  fd.timeout,
  fd.default_values,
  features.created,
  features.last_modified
FROM features
JOIN feature_data fd ON features.name = fd.name AND features.version = fd.version
ORDER BY features.name
;

-- name: FeatureGetForEnv :many
SELECT fd.*, features.created, features.last_modified
FROM features
JOIN feature_data fd ON features.name = fd.name AND features.version = fd.version
WHERE @environment_kind::text = ANY(environment_kinds)
ORDER BY features.name;

-- name: FeaturesForKind :many
SELECT
  fd.name,
  fd.version,
  fd.chart,
  fd.description,
  fd.source,
  fd.kinds::text[] AS kinds,
  fd.dependencies,
  fd.values,
  fd.timeout,
  fd.default_values,
  features.created,
  features.last_modified
FROM features
JOIN feature_data fd ON features.name = fd.name AND features.version = fd.version
WHERE @environment_kind::text = ANY(environment_kinds)
ORDER BY features.name;

-- name: FeatureVersionUpdate :exec
UPDATE features
SET version = @version
WHERE name = @name;