-- name: PartnerGet :one
SELECT *
FROM partners
WHERE id = @id;

-- name: PartnersGet :many
SELECT *
FROM partners
ORDER BY created DESC, name ASC;

-- name: PartnerCreate :one
INSERT INTO partners (name, description) VALUES (@name, @description) RETURNING *;

-- name: PartnerEnvironments :many
SELECT e.*, p.name AS partner_name
FROM environments e
JOIN partners p ON e.partner_id = p.id
ORDER BY p.name, e.name;
