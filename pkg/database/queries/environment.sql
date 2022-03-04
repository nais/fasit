-- name: EnvironmentGet :one
SELECT *
FROM environments
WHERE id = @id;

-- name: EnvironmentsGet :many
SELECT *
FROM environments
WHERE partner_id = @partner_id;

-- name: EnvironmentCreate :one
INSERT INTO environments (name, description, partner_id) VALUES (@name, @description, @partner_id) RETURNING *;