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

-- name: GetByName :one
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
	AND name = @name;

-- name: List :many
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
ORDER BY
	CASE WHEN name = 'management' THEN
		1
	ELSE
		2
	END,
	name ASC;

