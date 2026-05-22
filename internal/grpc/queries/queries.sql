-- name: CreateTenant :one
INSERT INTO tenants(
	name,
	description)
VALUES (
	@name,
	@description)
RETURNING
	*;

-- name: GetTenantByName :one
SELECT
	*
FROM
	tenants
WHERE
	name = @name;

-- name: GetTenant :one
SELECT
	*
FROM
	tenants
WHERE
	id = @id;

-- name: CreateEnvironment :one
INSERT INTO environments(
	name,
	description,
	tenant_id,
	kind,
	labels)
VALUES (
	@name,
	@description,
	@tenant_id,
	@kind,
	@labels)
RETURNING
	*;

-- name: GetEnvironment :one
SELECT
	*
FROM
	environments
WHERE
	id = @id;

-- name: SetEnvironmentLabels :exec
UPDATE
	environments
SET
	labels = @labels
WHERE
	id = @id;

-- name: GetEnvironmentByName :one
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
	AND name = @name;

-- name: SetEnvironmentValue :exec
INSERT INTO environment_values(
	"environment_id",
	"key",
	"value",
	"secret")
VALUES (
	@environment_id,
	@key,
	@value,
	@secret)
ON CONFLICT (
	"environment_id",
	"key")
	DO UPDATE SET
		"value" = @value,
		"secret" = @secret;

-- name: GetEnvironmentValue :one
SELECT
	"environment_id",
	"key",
	"secret",
(
		CASE WHEN secret THEN
			CASE WHEN @show_sensitive::BOOL THEN
				value
			ELSE
				'"*****"'
			END
		ELSE
			value
		END)::JSONB AS "value"
FROM
	environment_values
WHERE
	"environment_id" = @environment_id
	AND "key" = @key;

