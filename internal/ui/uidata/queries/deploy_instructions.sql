-- name: ListDeployInstructions :many
SELECT
	di.*,
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

