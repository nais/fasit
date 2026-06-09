-- name: LogsByDeployInstruction :many
SELECT
	*
FROM
	logs
WHERE
	deploy_instruction = @deploy_instruction
ORDER BY
	TIME ASC;

-- name: LogsByID :one
SELECT
	*
FROM
	logs
WHERE
	id = @id;

