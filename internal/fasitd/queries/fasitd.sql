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

-- name: ListCommandOutcomeCounts :many
SELECT
	c.environment_id,
	latest.status,
	count(*)::BIGINT AS count
FROM
	fasitd_commands c
	CROSS JOIN LATERAL (
		SELECT
			s.status
		FROM
			fasitd_command_statuses s
		WHERE
			s.diid = c.diid
		ORDER BY
			s.created DESC,
			s.id DESC
		LIMIT 1) latest
WHERE
	c.created >= @since
	AND (sqlc.narg(environment_id)::UUID IS NULL
		OR c.environment_id = sqlc.narg(environment_id))
GROUP BY
	c.environment_id,
	latest.status
ORDER BY
	1,
	2;

-- name: ListReleaseComparison :many
SELECT
	COALESCE(f.environment_id, n.environment_id)::UUID AS environment_id,
	COALESCE(f.feature, n.feature)::TEXT AS feature,
	f.version AS fasitd_version,
	f.status AS fasitd_status,
	n.version AS naisd_version,
	n.status AS naisd_status
FROM
	fasitd_release_statuses f
	FULL OUTER JOIN release_statuses n ON n.environment_id = f.environment_id
	AND n.feature = f.feature
WHERE
	sqlc.narg(environment_id)::UUID IS NULL
	OR COALESCE(f.environment_id, n.environment_id) = sqlc.narg(environment_id)
ORDER BY
	2;

-- name: ListReleaseInventoryReports :many
SELECT
	environment_id,
	count(*)::BIGINT AS releases,
	max(created)::TIMESTAMPTZ AS reported_at
FROM
	fasitd_release_statuses
WHERE
	sqlc.narg(environment_id)::UUID IS NULL
	OR environment_id = sqlc.narg(environment_id)
GROUP BY
	environment_id
ORDER BY
	1;

