-- name: RolloutCreate :one
INSERT INTO rollouts (feature_name, chart, version) VALUES (@feature_name, @chart, @version) RETURNING *;