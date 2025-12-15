-- name: ClusterUpgradesCreate :one
INSERT INTO
	cluster_upgrades (
		"tenant_id",
		"environment_id",
		"version",
		"is_automatic"
	)
VALUES
	(@tenantId, @envID, @version, @isAutomatic)
RETURNING
	*
;

-- name: ClusterUpgradesGet :many
SELECT
	*
FROM
	cluster_upgrades
WHERE
	tenant_id = @tenantId
	AND environment_id = @envID
	AND status NOT IN ('DONE', 'FAILED')
ORDER BY
	last_modified DESC
FOR UPDATE
	SKIP LOCKED
;

-- name: ClusterUpgradesSetSlackMessage :one
UPDATE cluster_upgrades
SET
	"slack_message_timestamp" = @slackMessageTimestamp,
	"slack_channel_id" = @slackChannelID
WHERE
	"id" = @id
RETURNING
	*
;

-- name: ClusterUpgradesUpdateStatus :one
UPDATE cluster_upgrades
SET
	"status" = @status::cluster_upgrades_status,
	"upgrade_start_time" = CASE
		WHEN (
			@status::TEXT = 'CONTROL_PLANE_UPGRADE'
			OR @status::TEXT = 'NODE_UPGRADE'
		)
		AND upgrade_start_time IS NULL THEN NOW()
		ELSE upgrade_start_time
	END
WHERE
	"id" = @id
RETURNING
	*
;

-- name: ClusterUpgradesGetByID :one
SELECT
	*
FROM
	cluster_upgrades
WHERE
	id = @id
;

-- name: ClusterUpgradesHistoryGetByEnvironmentID :many
SELECT
	*
FROM
	cluster_upgrades
WHERE
	tenant_id = @tenantId
	AND environment_id = @envID
ORDER BY
	last_modified DESC
OFFSET
	@historyOffset
LIMIT
	@historyLimit
;

-- name: ClusterUpgradesGetByVersion :one
SELECT
	*
FROM
	cluster_upgrades
WHERE
	tenant_id = @tenantId
	AND environment_id = @envID
	AND version = @version
ORDER BY
	last_modified DESC
LIMIT
	1
;

-- name: ClusterUpgradesHistoryGetByTenantID :many
SELECT
	*
FROM
	cluster_upgrades
WHERE
	tenant_id = @tenantId
ORDER BY
	last_modified DESC
OFFSET
	@historyOffset
LIMIT
	@historyLimit
;

-- name: ClusterUpgradesHistoryGetAll :many
SELECT
	*
FROM
	cluster_upgrades
ORDER BY
	last_modified DESC
OFFSET
	@historyOffset
LIMIT
	@historyLimit
;

-- name: ClusterUpgradesCountByEnvironmentID :one
SELECT
	COUNT(*)
FROM
	cluster_upgrades
WHERE
	tenant_id = @tenantId
	AND environment_id = @envID
;

-- name: ClusterUpgradesCountByTenantID :one
SELECT
	COUNT(*)
FROM
	cluster_upgrades
WHERE
	tenant_id = @tenantId
;

-- name: ClusterUpgradesCountAll :one
SELECT
	COUNT(*)
FROM
	cluster_upgrades
;

-- name: ClusterUpgradesBypassDelay :one
UPDATE cluster_upgrades
SET
	"status" = 'CREATED'::cluster_upgrades_status,
	"is_automatic" = FALSE
WHERE
	"id" = @id
	AND "status" = 'WAITING'::cluster_upgrades_status
RETURNING
	*
;
