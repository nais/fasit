-- name: EnvironmentGet :one
SELECT
	*
FROM
	environments
WHERE
	id = @id
;

-- name: EnvironmentGetByName :one
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
	AND name = @name
;

-- name: EnvironmentsGet :many
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
ORDER BY
	CASE
		WHEN name = 'management' THEN 1
		ELSE 2
	END,
	name ASC
;

-- name: EnvironmentByNames :one
SELECT
	e.*
FROM
	tenants t
	JOIN environments e ON e.tenant_id = t.id
	AND e.name = @environment_name
WHERE
	t.name = @tenant_name
LIMIT
	1
;

-- name: EnvironmentIDByNames :one
SELECT
	e.id
FROM
	tenants p
	JOIN environments e ON e.tenant_id = p.id
	AND e.name = @environment_name
WHERE
	p.name = @tenant_name
LIMIT
	1
;

-- name: EnvironmentCreate :one
INSERT INTO
	environments (name, description, tenant_id, kind)
VALUES
	(@name, @description, @tenant_id, @kind)
RETURNING
	*
;

-- name: EnvironmentUpdate :one
UPDATE environments
SET
	description = @description
WHERE
	id = @id
RETURNING
	*
;

-- name: EnvironmentCI :one
SELECT
	*
FROM
	environments
WHERE
	ci = TRUE
	AND kind = @kind
;

-- name: EnvironmentSetReconcile :one
UPDATE environments
SET
	reconcile = @reconcile
WHERE
	id = @id
RETURNING
	*
;

-- name: EnvironmentSetAutoUpgrade :one
UPDATE environments
SET
	auto_upgrade = @auto_upgrade
WHERE
	id = @id
RETURNING
	*
;

-- name: EnvironmentSetUpgradeDelayDays :one
UPDATE environments
SET
	upgrade_delay_days = @upgrade_delay_days
WHERE
	id = @id
RETURNING
	*
;

-- name: EnvironmentSetMaintenanceWindow :one
UPDATE environments
SET
	maintenance_window = @maintenance_window
WHERE
	id = @id
RETURNING
	*
;

-- name: EnvironmentsGetByAutoUpgrade :many
SELECT
	*
FROM
	environments
WHERE
	auto_upgrade = TRUE
ORDER BY
	CASE
		WHEN name = 'management' THEN 1
		ELSE 2
	END,
	name ASC
;

-- name: EnvironmentSetLabels :exec
UPDATE environments
SET
	labels = @labels
WHERE
	id = @id
;

-- name: EnvironmentGetLabels :one
SELECT
	labels
FROM
	environments
WHERE
	id = @id
;
