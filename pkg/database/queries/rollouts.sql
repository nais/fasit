-- name: RolloutUpdate :one
UPDATE rollouts
SET status = @status
  , changeset = @changeset
WHERE
  id = @id
RETURNING *;

-- name: RolloutsUnprocessed :many
SELECT * FROM rollouts WHERE status = '';

-- name: RolloutCreate :one
INSERT INTO rollouts (rollout_summary_id, feature, kind, changeset)
VALUES (@rolloutSummaryID, @feature, @envKind, @changeset)
RETURNING *;

-- name: RolloutGetByID :one
SELECT * FROM rollouts WHERE id = @id;

-- name: RolloutsUpdateStatus :exec
UPDATE rollouts
SET status = @status
WHERE
  id = ANY(@ids::uuid[])
AND status = 'pending'
;

-- name: RolloutEventCreate :exec
INSERT INTO rollout_events (rollout_id, type, data)
VALUES (@rollout_id, @type, @data)
;

-- name: RolloutEventsGetByRolloutID :many
SELECT * FROM rollout_events WHERE rollout_id = @rollout_id ORDER BY created ASC;

-- name: RolloutSummaryCreate :one
INSERT INTO rollout_summaries (feature)
VALUES (@feature)
RETURNING *;

-- name: RolloutsBySummaryID :many
SELECT * FROM rollouts WHERE rollout_summary_id = @rollout_summary_id;

-- name: RolloutSummaryDone :one
SELECT count(1) = 0 as incomplete FROM rollouts WHERE rollout_summary_id = @rollout_summary_id AND status != 'deployed';
