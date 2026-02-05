-- name: DeployInstructionsByID :one
SELECT
	*
FROM
	deploy_instructions
WHERE
	id = @id;

-- name: CreateDeployInstruction :one
INSERT INTO deploy_instructions(
	environment_id,
	feature_name,
	feature_version,
	hash,
	"values",
	deployment_id)
VALUES (
	@environment_id,
	@feature_name,
	@feature_version,
	@hash,
	@values,
	@deployment_id)
RETURNING
	id;

