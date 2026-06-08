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

-- name: ListLatestDeploys :many
SELECT
	environment_id,
	feature_name,
	hash,
	status,
	feature_assignment_id
FROM
	deploy_status
ORDER BY
	environment_id,
	feature_name;

-- name: ListDeployedFeatures :many
SELECT
	environment_id,
	feature_name
FROM
	deploy_status
WHERE
	status = 'deployed'
ORDER BY
	environment_id,
	feature_name;

-- name: ListLatestDecisions :many
SELECT
	environment_id,
	feature_name,
	feature_assignment_id,
	feature_version,
	action,
	message
FROM
	decision_status
ORDER BY
	environment_id,
	feature_name;

-- name: AppendDecisions :copyfrom
INSERT INTO decision_log(
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	action,
	message)
VALUES (
	@environment_id,
	@feature_assignment_id,
	@feature_name,
	@feature_version,
	@action,
	@message);

-- name: AppendDeploys :copyfrom
INSERT INTO deploy_log(
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash,
	"values")
VALUES (
	@diid,
	@environment_id,
	@feature_assignment_id,
	@feature_name,
	@feature_version,
	@status,
	@hash,
	@vals);

-- name: AppendDeployStatus :exec
INSERT INTO deploy_log(
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash)
VALUES (
	@diid,
	@environment_id,
	@feature_assignment_id,
	@feature_name,
	@feature_version,
	@status,
	@hash);

-- name: LatestDeployByDIID :one
SELECT
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	hash,
	created
FROM
	deploy_log
WHERE
	diid = @diid
ORDER BY
	created DESC,
	id DESC
LIMIT 1;

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

-- name: TimeoutPendingDeploys :exec
INSERT INTO deploy_log(
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash)
SELECT
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	'failed',
	hash
FROM
	deploy_status
WHERE
	status IN ('sent', 'installing')
	AND created < NOW() - INTERVAL '1 hour';

