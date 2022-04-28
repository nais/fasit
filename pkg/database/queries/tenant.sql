-- name: TenantGet :one
SELECT *
FROM tenants
WHERE id = @id;

-- name: TenantsGet :many
SELECT *
FROM tenants
ORDER BY created DESC, name ASC;

-- name: TenantCreate :one
INSERT INTO tenants (name, description) VALUES (@name, @description) RETURNING *;

-- name: TenantEnvironments :many
SELECT e.*, p.name AS tenant_name
FROM environments e
JOIN tenants p ON e.tenant_id = p.id
ORDER BY p.name, e.name;
