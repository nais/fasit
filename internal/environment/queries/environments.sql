-- name: Create :one
INSERT INTO environments(
	name,
	description,
	tenant_id,
	kind)
VALUES (
	@name,
	@description,
	@tenant_id,
	@kind)
RETURNING
	*;

-- name: SetLabels :exec
UPDATE
	environments
SET
	labels = @labels
WHERE
	id = @id;

-- name: Get :one
SELECT
	*
FROM
	environments
WHERE
	id = @id;

-- name: GetLabels :one
SELECT
	labels
FROM
	environments
WHERE
	id = @id;

