-- name: ListEnvConfigByFeature :many
SELECT
	*
FROM
	configurations_environment
WHERE
	feature = @feature
	AND environment_id = @environment_id
ORDER BY
	key ASC;

-- name: ListGlobalConfigByFeature :many
SELECT
	*
FROM
	configurations_global
WHERE
	feature = @feature
ORDER BY
	key ASC;

-- name: GetEnvConfigByKey :one
SELECT
	*
FROM
	configurations_environment
WHERE
	environment_id = @environment_id
	AND feature = @feature
	AND key = @key;

-- name: ListAllEnvConfigByFeature :many
SELECT
	*
FROM
	configurations_environment
WHERE
	feature = @feature
ORDER BY
	environment_id,
	key ASC;

-- name: GetGlobalConfigByKey :one
SELECT
	*
FROM
	configurations_global
WHERE
	feature = @feature
	AND key = @key;

-- name: UpsertEnvConfig :one
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

-- name: UpsertGlobalConfig :one
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

-- name: UpdateGlobalConfig :one
UPDATE
	configurations_global
SET
	description = @description,
	value = @value
WHERE
	id = @id
RETURNING
	*;

-- name: DeleteGlobalConfig :exec
DELETE FROM configurations_global
WHERE id = @id;

-- name: GetGlobalConfigByID :one
SELECT
	*
FROM
	configurations_global
WHERE
	id = @id;

-- name: GetEnvConfigByID :one
SELECT
	*
FROM
	configurations_environment
WHERE
	id = @id;

-- name: UpdateEnvConfig :one
UPDATE
	configurations_environment
SET
	description = @description,
	value = @value
WHERE
	id = @id
RETURNING
	*;

-- name: DeleteEnvConfig :exec
DELETE FROM configurations_environment
WHERE id = @id;

