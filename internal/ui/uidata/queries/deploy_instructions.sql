-- Deploy history for the UI, derived from the deploy_status view (latest row
-- per environment x feature). One row per environment for the assignment.
-- name: ListDeployInstructions :many
SELECT
	ds.diid AS id,
	ds.environment_id,
	ds.feature_assignment_id,
	ds.feature_name,
	ds.feature_version,
	ds.status,
	ds.hash,
	ds.created,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	deploy_status ds
	JOIN environments e ON e.id = ds.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	ds.feature_assignment_id = @feature_assignment_id::UUID
ORDER BY
	ds.created DESC;

-- name: GetDeployInstructionByFeatureAssignmentAndEnvironmentID :one
SELECT
	ds.diid AS id,
	ds.environment_id,
	ds.feature_assignment_id,
	ds.feature_name,
	ds.feature_version,
	ds.status,
	ds.hash,
	ds.created,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	deploy_status ds
	JOIN environments e ON e.id = ds.environment_id
	JOIN tenants t ON t.id = e.tenant_id
WHERE
	ds.feature_assignment_id = @feature_assignment_id::UUID
	AND ds.environment_id = @environment_id::UUID;

