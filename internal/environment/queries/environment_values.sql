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

-- name: DeleteEnvironmentValue :exec
DELETE FROM environment_values
WHERE "environment_id" = @environment_id
	AND "key" = @key;

