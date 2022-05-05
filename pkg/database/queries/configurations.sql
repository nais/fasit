-- name: ConfigGet :many
SELECT *
FROM ONLY configurations_global
WHERE feature = @feature;

-- name: ConfigEnvUpdateOrCreate :one
INSERT INTO configurations_environment
	(environment_id, feature, description, secret, key, value)
VALUES
	(@environment_id, @feature, @description, @secret, @key, @value)
ON CONFLICT (environment_id, feature, key) DO UPDATE
	SET
		value = EXCLUDED.value,
		description = EXCLUDED.description
RETURNING *;

-- name: ConfigGlobalUpdateOrCreate :one
INSERT INTO configurations_global
	(feature, description, secret, key, value)
VALUES
	(@feature, @description, @secret, @key, @value)
ON CONFLICT (feature, key) DO UPDATE
	SET
		value = EXCLUDED.value,
		description = EXCLUDED.description
RETURNING *;

-- name: ConfigGetForEnv :many
SELECT *
FROM configurations_environment
WHERE feature = @feature AND environment_id = @environment_id;

-- name: EnvConfig :many
WITH "combined" AS (
		SELECT *, NULL::uuid AS environment_id
		FROM ONLY configurations_global glob
		WHERE glob.feature = @feature

		UNION

		SELECT *
		FROM ONLY configurations_environment env
		WHERE env.feature = @feature
		AND environment_id = @environment_id
	), "filtered" AS (
		SELECT *, RANK() OVER (
				PARTITION BY "key"
				ORDER BY environment_id ASC, key ASC
			)
		FROM "combined"
	)
SELECT *
FROM filtered
WHERE RANK = 1
;

-- name: ConfigUpdate :one
UPDATE configurations_global
SET description = @description,
	value = @value
WHERE id = @id
RETURNING *;

-- name: ConfigDelete :exec
DELETE FROM configurations_global
WHERE id = @id;
