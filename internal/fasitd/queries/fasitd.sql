-- name: CreateCommand :exec
INSERT INTO fasitd_commands(
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	chart,
	config_hash,
	uninstall,
	"values")
VALUES (
	@diid,
	@environment_id,
	@feature_assignment_id,
	@feature_name,
	@feature_version,
	@chart,
	@config_hash,
	@uninstall,
	@vals);

-- name: AppendCommandStatus :exec
INSERT INTO fasitd_command_statuses(
	diid,
	status,
	message)
VALUES (
	@diid,
	@status,
	@message);

-- name: GetCommandByDIID :one
SELECT
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	chart,
	config_hash,
	uninstall,
	created
FROM
	fasitd_commands
WHERE
	diid = @diid;

-- name: AppendHelmLogs :batchexec
INSERT INTO fasitd_helm_logs(
	diid,
	TIME,
	message,
	kind)
VALUES (
	@diid,
	@time,
	@message,
	@kind);

-- name: DeleteReleaseStatusesInEnvironment :exec
DELETE FROM fasitd_release_statuses
WHERE environment_id = @environment_id;

-- name: SetReleaseStatus :exec
INSERT INTO fasitd_release_statuses(
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

