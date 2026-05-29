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

-- name: ListLatestDeployments :many
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
	deployments d
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
	deployment_id
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
	deployment_id)
VALUES (
	@id,
	@environment_id,
	@feature_name,
	@feature_version,
	@hash,
	@vals,
	@deployment_id);

-- name: UpsertDeploymentStatus :batchexec
INSERT INTO deployment_statuses(
	deployment_id,
	environment_id,
	status,
	message)
VALUES (
	@deployment_id,
	@environment_id,
	@status,
	@message)
ON CONFLICT (
	deployment_id,
	environment_id)
	DO UPDATE SET
		status = EXCLUDED.status,
		message = EXCLUDED.message;

-- name: SetDeployInstructionStatus :exec
UPDATE
	deploy_instructions
SET
	status = @status
WHERE
	id = @id
	AND status = 'created';

