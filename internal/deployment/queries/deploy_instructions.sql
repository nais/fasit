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

-- name: TimeoutDeployInstructions :exec
UPDATE
	deploy_instructions
SET
	status = 'failed'
WHERE
	status = 'pending'
	AND last_modified < NOW() - INTERVAL '1 hour';

-- name: GetDeploymentStatusLog :many
SELECT
	sqlc.embed(di),
	e.name AS environment_name,
	t.name AS tenant_name,
	sqlc.embed(l)
FROM
	deploy_instructions di
	JOIN environments e ON e.id = di.environment_id
	JOIN tenants t ON t.id = e.tenant_id
	JOIN logs l ON l.deploy_instruction = di.id
WHERE
	di.deployment_id = @deployment_id
	AND di.environment_id = @environment_id
ORDER BY
	l.time ASC;

-- name: GetDeployInstructionByDeploymentAndEnvironmentID :one
SELECT
	sqlc.embed(di),
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	deploy_instructions di
	JOIN environments e ON e.id = di.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	di.deployment_id = @deployment_id
	AND di.environment_id = @environment_id;

-- name: ListDeployInstructionsByDeploymentID :many
SELECT
	sqlc.embed(di),
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	deploy_instructions di
	JOIN environments e ON e.id = di.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	di.deployment_id = @deployment_id
ORDER BY
	di.created DESC;

