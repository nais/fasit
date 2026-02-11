-- name: DeployInstructionsCreate :one
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

-- name: DeployInstructionsLatestForEnvironment :many
SELECT
	*
FROM
	deploy_instructions
WHERE
	id IN ( SELECT DISTINCT ON (feature_name)
			id
		FROM
			deploy_instructions di
		WHERE
			di.environment_id = @environment_id
		ORDER BY
			feature_name,
			created DESC);

-- name: RolloutAssignDeployInstruction :exec
UPDATE
	rollouts
SET
	deploy_instructions = ARRAY_APPEND(deploy_instructions, @deploy_instruction_id)
WHERE
	feature_name = @feature_name
	AND version = @version
	AND status = 'pending';

