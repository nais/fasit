-- name: LogsByDeployInstruction :many
SELECT
	*
FROM
	logs
WHERE
	deploy_instruction = @deploy_instruction
ORDER BY
	TIME ASC;

-- name: LogsByDeployInstructions :many
SELECT
	*
FROM
	logs
WHERE
	deploy_instruction = ANY (@deploy_instructions::UUID[])
ORDER BY
	deploy_instruction,
	TIME ASC;

-- name: LogsByID :one
SELECT
	*
FROM
	logs
WHERE
	id = @id;

