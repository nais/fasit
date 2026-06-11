-- name: CreateEnvironment :one
INSERT INTO environments(
	name,
	description,
	tenant_id,
	kind,
	labels)
VALUES (
	@name,
	@description,
	@tenant_id,
	@kind,
	@labels)
RETURNING
	*;

-- name: SetEnvironmentLabels :exec
UPDATE
	environments
SET
	labels = @labels
WHERE
	id = @id;

-- name: SetEnvironmentOIDC :exec
UPDATE
	environments
SET
	oidc_issuer = @oidc_issuer,
	oidc_discovery_url = @oidc_discovery_url
WHERE
	id = @id;

-- name: GetEnvironment :one
SELECT
	*
FROM
	environments
WHERE
	id = @id;

-- name: GetEnvironmentLabels :one
SELECT
	labels
FROM
	environments
WHERE
	id = @id;

-- name: GetEnvironmentByName :one
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
	AND name = @name;

-- name: ListEnvironments :many
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

