-- name: GetTenant :one
SELECT
	*
FROM
	tenants
WHERE
	id = @id;

-- name: ListTenants :many
SELECT
	*
FROM
	tenants
ORDER BY
	created DESC,
	name ASC;

-- name: GetTenantByName :one
SELECT
	*
FROM
	tenants
WHERE
	name = @name;

-- name: CreateTenant :one
INSERT INTO tenants(
	name,
	description)
VALUES (
	@name,
	@description)
RETURNING
	*;

-- name: ListTenantEnvironments :many
SELECT
	e.*,
	t.name AS tenant_name
FROM
	environments e
	JOIN tenants t ON e.tenant_id = t.id
WHERE
	-- If @all is false, only return environments with reconcile enabled
	CASE WHEN TRUE = sqlc.arg('all')::BOOLEAN THEN
		TRUE
	ELSE
		e.reconcile = TRUE
	END
ORDER BY
	t.name,
	e.name;

