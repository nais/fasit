-- name: RolloutDelete :exec
DELETE FROM rollouts WHERE feature_name = @feature_name;

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
WHERE @environment_kind::text = ANY(kinds::text[])
ORDER BY rollouts.feature_name;

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
AND rollouts.status = 'pending'
;

-- name: RolloutUpdateStatus :exec
UPDATE rollouts SET status = @status WHERE feature_name = @feature_name and completed IS NULL;

-- name: RolloutEventCreate :exec
INSERT INTO rollout_events (rollout_id, failure, message) VALUES (@rollout_id, @failure::boolean, @message);

-- name: RolloutStatus :one
SELECT status FROM rollouts WHERE feature_name = @feature_name and completed IS NULL;

-- name: RolloutComplete :exec
UPDATE rollouts SET completed = NOW() WHERE feature_name = @feature_name and completed IS NULL;
