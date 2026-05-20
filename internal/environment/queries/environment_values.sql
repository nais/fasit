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
			CASE WHEN @showSensitive::BOOL THEN
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

-- name: ListEnvironmentValuesForEnvironment :many
SELECT
	"environment_id",
	"environment_values"."key",
	"secret",
(
		CASE WHEN secret THEN
			CASE WHEN @showSensitive::BOOL THEN
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
ORDER BY
	"environment_values"."key" ASC;

-- name: ListEnvironmentValuesForKey :many
SELECT
	ev.environment_id,
	ev.key,
	ev.secret,
	ev.value,
	t.id AS tenant_id,
	t.name AS tenant_name,
	e.name AS environment_name
FROM
	environment_values ev
	JOIN environments e ON e.id = ev.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	ev.key = @key
ORDER BY
	e.name ASC;

-- name: DeleteEnvironmentValue :exec
DELETE FROM environment_values
WHERE "environment_id" = @environment_id
	AND "key" = @key;

