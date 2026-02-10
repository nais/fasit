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

-- name: GetLatestDeployInstructionsForFeature :one
SELECT
	*
FROM
	deploy_instructions
WHERE
	feature_name = @feature_name
	AND environment_id = @environment_id
ORDER BY
	created DESC
LIMIT 1;

