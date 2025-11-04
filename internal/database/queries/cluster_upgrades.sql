-- name: ClusterUpgradesCreate :one
INSERT INTO
	cluster_upgrades ("tenant_id", "environment_id", "version")
VALUES
	(@tenantId, @envID, @version)
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
	"status" = @status
WHERE
	"tenant_id" = @tenantId
	AND "environment_id" = @envID
	AND "version" = @version
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
