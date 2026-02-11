-- name: TenantGet :one
SELECT
	*
FROM
	tenants
WHERE
	id = @id;

-- name: TenantGetByName :one
SELECT
	*
FROM
	tenants
WHERE
	name = @name;

-- name: TenantsGet :many
SELECT
	*
FROM
	tenants
ORDER BY
	created DESC,
	name ASC;

-- name: TenantCreate :one
INSERT INTO tenants(
	name,
	description)
VALUES (
	@name,
	@description)
RETURNING
	*;

-- name: TenantSetUpgradeDelayDays :one
UPDATE
	tenants
SET
	upgrade_delay_days = @upgrade_delay_days
WHERE
	id = @id
RETURNING
	*;

