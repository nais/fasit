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
