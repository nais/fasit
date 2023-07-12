-- name: LogsCreate :batchexec
INSERT INTO
  logs
(
  deploy_instruction,
  time,
  message
)
VALUES (
  @deploy_instruction,
  @time,
  @message
);
