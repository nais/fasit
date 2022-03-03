-- name: ConfigGet :one
SELECT *
FROM configurations
WHERE feature = @feature AND environment_id IS NULL;

-- name: ConfigCreate :one
INSERT INTO configurations (environment_id, feature, description, secret, key, value) VALUES (@environment_id, @feature, @description, @secret, @key, @value) RETURNING *;

