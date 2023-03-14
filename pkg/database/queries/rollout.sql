-- name: RolloutCreate :one
INSERT INTO rollouts (feature_name, version) VALUES (@feature_name, @version) RETURNING *;

-- name: RolloutsForKind :many
SELECT
  rollouts.id,
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
  rollouts.created
FROM rollouts
JOIN feature_data fd ON rollouts.feature_name = fd.name AND rollouts.version = fd.version
WHERE @environment_kind::text = ANY(environment_kinds)
ORDER BY rollouts.features_name;

-- name: RolloutByName :one
SELECT
  rollouts.id,
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
  rollouts.created
FROM rollouts
JOIN feature_data fd ON rollouts.feature_name = fd.name AND rollouts.version = fd.version
WHERE fd.name = @name
;
