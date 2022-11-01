-- name: ConfigGet :many
SELECT *
FROM ONLY configurations_global
WHERE feature = @feature;

-- name: ConfigEnvUpdateOrCreate :one
INSERT INTO configurations_environment
	(environment_id, feature, description, secret, key, value, rollout_id)
VALUES
	(@environment_id, @feature, @description, @secret, @key, @value, @rollout_id)
ON CONFLICT (environment_id, feature, key) DO UPDATE
	SET
		value = EXCLUDED.value,
		description = EXCLUDED.description,
		rollout_id = EXCLUDED.rollout_id
RETURNING *;

-- name: ConfigGlobalUpdateOrCreate :one
INSERT INTO configurations_global
	(feature, description, secret, key, value, rollout_id)
VALUES
	(@feature, @description, @secret, @key, @value, @rollout_id)
ON CONFLICT (feature, key) DO UPDATE
	SET
		value = EXCLUDED.value,
		description = EXCLUDED.description,
		rollout_id = EXCLUDED.rollout_id
RETURNING *;

-- name: ConfigGetForEnv :many
SELECT *
FROM configurations_environment
WHERE feature = @feature AND environment_id = @environment_id;

-- name: EnvConfig :many
WITH "combined" AS (
		SELECT "id", "feature", "key", "value", "rollout_id", NULL::uuid AS environment_id
		FROM ONLY configurations_global glob
		WHERE glob.feature = @feature
		AND glob.key != ALL(@excludeKeys::text[])

		UNION

		SELECT "id", "feature", "key", "value", "rollout_id", "environment_id"
		FROM ONLY configurations_environment env
		WHERE env.feature = @feature
		AND environment_id = @environment_id
		AND env.key != ALL(@excludeKeys::text[])
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
	value = @value,
	rollout_id = NULL
WHERE id = @id
RETURNING *;

-- name: ConfigDelete :exec
DELETE FROM configurations_global
WHERE id = @id;

-- name: ConfigDeleteByRolloutID :exec
DELETE FROM configurations_environment
WHERE rollout_id = @rollout_id;

-- name: ConfigOverridesByFeature :many
SELECT environment_id, array_agg(key)::text[] AS keys
FROM configurations_environment
WHERE feature = @feature
GROUP BY environment_id
;
