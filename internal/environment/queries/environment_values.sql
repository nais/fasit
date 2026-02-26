-- name: SetEnvironmentValue :exec
INSERT INTO environment_values(
	"environment_id",
	"key",
	"value",
	"secret")
VALUES (
	@envID,
	@key,
	@value,
	@secret)
ON CONFLICT (
	"environment_id",
	"key")
	DO UPDATE SET
		"value" = @value,
		"secret" = @secret;

