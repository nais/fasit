-- name: FeatureStatesGet :many
SELECT *
FROM feature_states
WHERE environment_id = @environment_id;

-- name: FeatureStateGet :one
SELECT *
FROM feature_states
WHERE feature = @feature AND environment_id = @environment_id;

-- name: FeatureStateCreateOrUpdate :one
INSERT INTO feature_states
(environment_id, feature, enabled)
VALUES
    (@environment_id, @feature, @enabled)
ON CONFLICT (environment_id, feature) DO UPDATE
    SET
        enabled = EXCLUDED.enabled
RETURNING *;