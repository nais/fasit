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
  fd.default_values,
  fd.timeout,
  rollouts.created
FROM rollouts
JOIN feature_data fd ON rollouts.feature_name = fd.name AND rollouts.version = fd.version
WHERE @environment_kind::text = ANY(kinds::text[])
AND (
  rollouts.status = 'pending'
  -- Ensure that the latest rollout is returned if it's the only one remaining
  OR NOT EXISTS (
    SELECT 1
    FROM rollouts r2
    WHERE r2.feature_name = rollouts.feature_name
    AND r2.status IN ('pending', 'in_progress', 'deployed')
    AND r2.created > rollouts.created
  )
)
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
  fd.default_values,
  fd.timeout,
  rollouts.created
FROM rollouts
JOIN feature_data fd ON rollouts.feature_name = fd.name AND rollouts.version = fd.version
WHERE fd.name = @name
AND rollouts.status = 'pending';

-- name: RolloutUpdateStatus :exec
UPDATE rollouts SET status = @status WHERE feature_name = @feature_name and completed IS NULL;

-- name: RolloutEventCreate :exec
INSERT INTO rollout_events (rollout_id, failure, message, data) VALUES (@rollout_id, @failure::boolean, @message, @data);

-- name: RolloutStatus :one
SELECT status FROM rollouts WHERE feature_name = @feature_name and completed IS NULL;

-- name: RolloutComplete :exec
UPDATE rollouts SET completed = NOW() WHERE feature_name = @feature_name and completed IS NULL;

-- name: RolloutsForFeature :many
SELECT *
FROM rollouts
WHERE feature_name = @feature_name
ORDER BY created DESC
LIMIT 30
;

-- name: RolloutByNameAndVersion :one
SELECT *
FROM rollouts
WHERE feature_name = @feature_name
AND version = @version;

-- name: RolloutEventForRollout :many
SELECT *
FROM rollout_events
WHERE rollout_id = @rollout_id
ORDER BY created ASC;

-- name: RolloutCalculateDone :one
WITH rollout AS (
  SELECT * FROM rollouts WHERE rollouts.id = @rollout_id
), dis AS (
  SELECT di.*
  FROM deploy_instructions di
  INNER JOIN rollout ON di.feature_name = rollout.feature_name AND di.feature_version = rollout.version
  WHERE di.status IN ('deployed', 'failed')
), cienvs AS (
  SELECT id
  FROM environments
  WHERE ci = true
), feature_states AS (
  SELECT count(1)
  FROM feature_states
  WHERE feature = (SELECT feature_name FROM rollout)
  AND environment_id IN (SELECT id FROM cienvs)
  AND enabled = true
)
SELECT (
  SELECT count(1) FROM dis
) = (
  SELECT * FROM feature_states
) AS done
;
