-- name: RolloutCreate :one
INSERT INTO rollouts (feature_name, version) VALUES (@feature_name, @version) RETURNING *;

-- name: RolloutsForKind :many
SELECT feature_data.*, rollouts.created
FROM rollouts 
JOIN feature_data ON rollouts.feature_name = feature_data.name AND rollouts.version = feature_data.version
WHERE @environment_kind::text = ANY(environment_kinds) 
ORDER BY rollouts.features_name;