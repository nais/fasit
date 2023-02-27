-- name: RolloutCreate :one
INSERT INTO rollouts (feature_name, version) VALUES (@feature_name, @version) RETURNING *;

-- name: RolloutsForKind :many
SELECT * FROM rollouts WHERE @environment_kind::text = ANY(environment_kinds);