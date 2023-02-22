-- name: RolloutCreate :one
INSERT INTO rollouts (feature_name, chart, version) VALUES (@feature_name, @chart, @version) RETURNING *;

-- name: RolloutsForKind :many
SELECT * FROM rollouts WHERE @environment_kind::text = ANY(environment_kinds);