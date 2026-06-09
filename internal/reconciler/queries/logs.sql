-- name: LogsCreate :batchexec
INSERT INTO logs(
	deploy_instruction,
	TIME,
	message,
	kind)
VALUES (
	@deploy_instruction,
	@time,
	@message,
	@kind);

