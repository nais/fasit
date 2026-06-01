-- name: GetDeployInstruction :one
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

-- name: TimeoutDeployInstructions :exec
UPDATE
	deploy_instructions
SET
	status = 'failed'
WHERE
	status = 'pending'
	AND last_modified < NOW() - INTERVAL '1 hour';

-- name: GetDeployInstructionByFeatureAssignmentAndEnvironmentID :one
SELECT
	sqlc.embed(di),
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	deploy_instructions di
	JOIN environments e ON e.id = di.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	di.feature_assignment_id = @feature_assignment_id
	AND di.environment_id = @environment_id;

-- name: ListDeployInstructions :many
SELECT
	sqlc.embed(di),
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	deploy_instructions di
	JOIN environments e ON e.id = di.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	di.feature_assignment_id = @feature_assignment_id
ORDER BY
	di.created DESC;

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

