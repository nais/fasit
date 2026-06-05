-- name: CreateDeployInstruction :one
INSERT INTO deploy_instructions(
	environment_id,
	feature_name,
	feature_version,
	hash,
	"values",
	feature_assignment_id)
VALUES (
	@environment_id,
	@feature_name,
	@feature_version,
	@hash,
	@values,
	@feature_assignment_id)
RETURNING
	id;

-- name: InvalidateDeployInstruction :exec
UPDATE
	deploy_instructions
SET
	hash = '',
	status = 'invalidated'
WHERE
	id =(
		SELECT
			di.id
		FROM
			deploy_instructions di
		WHERE
			di.feature_name = @feature_name
			AND di.environment_id = @environment_id
		ORDER BY
			di.created DESC
		LIMIT 1);

-- name: UpdateDeployInstructionStatus :exec
UPDATE
	deploy_instructions
SET
	status = @status
WHERE
	id = @id;

