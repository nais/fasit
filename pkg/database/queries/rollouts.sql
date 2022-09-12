-- name: RolloutsGet :one
SELECT * FROM rollouts WHERE id = @id;

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
INSERT INTO rollouts (feature, changeset)
VALUES (@feature, @changeset)
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
