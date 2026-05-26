-- name: ConfigEnvByFeatureAndEnv :many
SELECT
	*
FROM
	configurations_environment
WHERE
	feature = @feature
	AND environment_id = @environment_id
ORDER BY
	key ASC;

-- name: ConfigGet :many
SELECT
	*
FROM
	configurations_global
WHERE
	feature = @feature
ORDER BY
	key ASC;

-- name: ConfigEnvGet :one
SELECT
	*
FROM
	configurations_environment
WHERE
	environment_id = @environment_id
	AND feature = @feature
	AND key = @key;

-- name: ConfigEnvListByFeature :many
SELECT
	*
FROM
	configurations_environment
WHERE
	feature = @feature
ORDER BY
	environment_id,
	key ASC;

-- name: ConfigGlobalGetByKey :one
SELECT
	*
FROM
	configurations_global
WHERE
	feature = @feature
	AND key = @key;

-- name: ConfigEnvUpdateOrCreate :one
INSERT INTO configurations_environment(
	environment_id,
	feature,
	description,
	secret,
	key,
	value)
VALUES (
	@environment_id,
	@feature,
	@description,
	@secret,
	@key,
	@value)
ON CONFLICT (
	environment_id,
	feature,
	key)
	DO UPDATE SET
		value = EXCLUDED.value,
		description = EXCLUDED.description
	RETURNING
		*;

-- name: ConfigGlobalUpdateOrCreate :one
INSERT INTO configurations_global(
	feature,
	description,
	secret,
	key,
	value)
VALUES (
	@feature,
	@description,
	@secret,
	@key,
	@value)
ON CONFLICT (
	feature,
	key)
	DO UPDATE SET
		value = EXCLUDED.value,
		description = EXCLUDED.description
	RETURNING
		*;

-- name: ConfigUpdate :one
UPDATE
	configurations_global
SET
	description = @description,
	value = @value
WHERE
	id = @id
RETURNING
	*;

-- name: ConfigDelete :exec
DELETE FROM configurations_global
WHERE id = @id;

-- name: ConfigOverridesByFeature :many
SELECT
	environment_id,
	ARRAY_AGG(key)::TEXT[] AS keys
FROM
	configurations_environment
WHERE
	feature = @feature
GROUP BY
	environment_id
ORDER BY
	environment_id ASC;

-- name: ConfigGetByID :one
SELECT
	*
FROM
	configurations_global
WHERE
	id = @id;

-- name: ConfigEnvGetByID :one
SELECT
	*
FROM
	configurations_environment
WHERE
	id = @id;

-- name: ConfigEnvUpdate :one
UPDATE
	configurations_environment
SET
	description = @description,
	value = @value
WHERE
	id = @id
RETURNING
	*;

-- name: ConfigEnvDelete :exec
DELETE FROM configurations_environment
WHERE id = @id;

-- name: ConfigRenameGlobal :exec
UPDATE
	configurations_global
SET
	key = @to_key
WHERE
	configurations_global.key = @from_key
	AND configurations_global.feature = @feature
	AND NOT EXISTS (
		SELECT
			1
		FROM
			configurations_global nested
		WHERE
			nested.feature = @feature
			AND nested.key = @to_key);

-- name: ConfigRenameEnv :exec
UPDATE
	configurations_environment
SET
	key = @to_key
WHERE
	configurations_environment.key = @from_key
	AND NOT EXISTS (
		SELECT
			1
		FROM
			configurations_environment nested
		WHERE
			configurations_environment.feature = @feature
			AND nested.key = @to_key
			AND nested.environment_id = configurations_environment.environment_id);

