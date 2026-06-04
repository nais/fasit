-- name: ListAllTenantEnvironments :many
SELECT
	e.id,
	e.name,
	e.kind,
	e.labels,
	e.reconcile,
	t.id AS tenant_id,
	t.name AS tenant_name
FROM
	environments e
	JOIN tenants t ON t.id = e.tenant_id
ORDER BY
	t.name,
	e.name;

-- name: ListHealthStatuses :many
SELECT
	environment_id,
	reported_at
FROM
	health_statuses
ORDER BY
	environment_id;

-- name: ListLatestFeatureAssignments :many
SELECT DISTINCT ON (d.feature_name, d.target)
	d.id,
	d.feature_name,
	d.version,
	d.target,
	d.created,
	fd.name AS fd_name,
	fd.version AS fd_version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds,
	fd.dependencies,
	fd."values",
	fd.default_values,
	fd.timeout,
	fd.tpl_details
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.active = TRUE
ORDER BY
	d.feature_name,
	d.target,
	d.created DESC;

-- name: ListDisabledFeatures :many
SELECT
	environment_id,
	feature
FROM
	disabled_features
ORDER BY
	environment_id,
	feature;

-- name: ListAllGlobalConfigs :many
SELECT
	id,
	feature,
	key,
	value,
	secret,
	created
FROM
	configurations_global
ORDER BY
	feature,
	key;

-- name: ListAllEnvConfigs :many
SELECT
	id,
	environment_id,
	feature,
	key,
	value,
	secret,
	created
FROM
	configurations_environment
ORDER BY
	environment_id,
	feature,
	key;

-- name: ListAllEnvironmentValues :many
SELECT
	ev.environment_id,
	ev.key,
	ev.value
FROM
	environment_values ev
ORDER BY
	ev.environment_id,
	ev.key;

-- name: ListLatestDeployInstructions :many
SELECT DISTINCT ON (feature_name, environment_id)
	id,
	environment_id,
	feature_name,
	hash,
	status,
	feature_assignment_id
FROM
	deploy_instructions
ORDER BY
	feature_name,
	environment_id,
	created DESC;

-- name: ListDeployedFeatures :many
SELECT DISTINCT ON (feature_name, environment_id)
	feature_name,
	environment_id
FROM
	deploy_instructions
WHERE
	status = 'deployed'
ORDER BY
	feature_name,
	environment_id;

-- name: CreateDeployInstruction :batchexec
INSERT INTO deploy_instructions(
	id,
	environment_id,
	feature_name,
	feature_version,
	hash,
	"values",
	feature_assignment_id)
VALUES (
	@id,
	@environment_id,
	@feature_name,
	@feature_version,
	@hash,
	@vals,
	@feature_assignment_id);

-- name: UpsertReconcileStatus :batchexec
INSERT INTO feature_reconcile_statuses(
	feature_assignment_id,
	environment_id,
	status,
	message)
VALUES (
	@feature_assignment_id,
	@environment_id,
	@status,
	@message)
ON CONFLICT (
	feature_assignment_id,
	environment_id)
	DO UPDATE SET
		status = EXCLUDED.status,
		message = EXCLUDED.message;

-- name: SetDeployInstructionStatusForCreated :exec
UPDATE
	deploy_instructions
SET
	status = @status
WHERE
	id = @id
	AND status = 'created';

-- name: SetDeployInstructionStatus :exec
UPDATE
	deploy_instructions
SET
	status = @status
WHERE
	id = @id;

-- name: GetDeployInstruction :one
SELECT
	*
FROM
	deploy_instructions
WHERE
	id = @id;

-- name: DeleteReleaseStatusesInEnvironment :exec
DELETE FROM release_statuses
WHERE environment_id = @environment_id;

-- name: SetReleaseStatus :exec
INSERT INTO release_statuses(
	environment_id,
	feature,
	version,
	status,
	revision,
	last_deployed)
VALUES (
	@environment_id,
	@feature,
	@version,
	@status,
	@revision,
	@last_deployed)
ON CONFLICT (
	environment_id,
	feature)
	DO UPDATE SET
		version = EXCLUDED.version,
		status = EXCLUDED.status,
		revision = EXCLUDED.revision,
		last_deployed = EXCLUDED.last_deployed;

-- name: SetReconcileStatus :exec
INSERT INTO feature_reconcile_statuses(
	feature_assignment_id,
	environment_id,
	status,
	message)
VALUES (
	@feature_assignment_id,
	@environment_id,
	@status,
	@message)
ON CONFLICT (
	feature_assignment_id,
	environment_id)
	DO UPDATE SET
		status = EXCLUDED.status,
		message = EXCLUDED.message;

-- name: TimeoutDeployInstructions :exec
UPDATE
	deploy_instructions
SET
	status = 'failed'
WHERE
	status = 'pending'
	AND last_modified < NOW() - INTERVAL '1 hour';

