-- name: EnvironmentValueStore :exec
INSERT INTO environment_values ("environment_id", "key", "value") VALUES (@envID, @key, @value)
ON CONFLICT ("environment_id", "key") DO UPDATE SET "value" = @value;

-- name: EnvironmentValueGet :one
SELECT * FROM environment_values WHERE "environment_id" = @envID AND "key" = @key;
