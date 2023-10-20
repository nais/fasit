-- name: LogsCreate :batchexec
INSERT INTO
  logs
(
  deploy_instruction,
  time,
  message,
  kind
)
VALUES (
  @deploy_instruction,
  @time,
  @message,
  @kind
)
;

-- name: LogsByDeployInstruction :many
SELECT *
FROM logs
WHERE deploy_instruction = @deploy_instruction
ORDER BY time
ASC
;


-- name: LogsByID :one
SELECT *
FROM logs
WHERE id = @id
;
