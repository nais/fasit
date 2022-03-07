-- name: EnvironmentGet :one
SELECT *
FROM environments
WHERE id = @id;

-- name: EnvironmentsGet :many
SELECT *
FROM environments
WHERE partner_id = @partner_id;

-- name: EnvironmentIDByNames :one
SELECT e.id
FROM partners p
JOIN environments e ON e.partner_id = p.id AND e.name = @environment_name
WHERE p.name = @partner_name
LIMIT 1;

-- name: EnvironmentCreate :one
INSERT INTO environments (name, description, partner_id) VALUES (@name, @description, @partner_id) RETURNING *;
