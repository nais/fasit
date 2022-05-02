-- name: EnvironmentGet :one
SELECT *
FROM environments
WHERE id = @id;

-- name: EnvironmentsGet :many
SELECT *
FROM environments
WHERE tenant_id = @tenant_id;

-- name: EnvironmentByNames :one
SELECT *
FROM tenants t
         JOIN environments e ON e.tenant_id = t.id AND e.name = @environment_name
WHERE t.name = @tenant_name
    LIMIT 1;

-- name: EnvironmentIDByNames :one
SELECT e.id
FROM tenants p
JOIN environments e ON e.tenant_id = p.id AND e.name = @environment_name
WHERE p.name = @tenant_name
LIMIT 1;

-- name: EnvironmentCreate :one
INSERT INTO environments (name, description, tenant_id, kind) VALUES (@name, @description, @tenant_id, @kind) RETURNING *;

-- name: EnvironmentUpdate :one
UPDATE environments
SET description = @description
WHERE
    id = @id
    RETURNING *;