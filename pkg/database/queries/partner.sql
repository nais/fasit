-- name: PartnerGet :one
SELECT *
FROM partners
WHERE id = @id;

-- name: PartnersGet :many
SELECT *
FROM partners;

-- name: PartnerCreate :one
INSERT INTO partners (name, description) VALUES (@name, @description) RETURNING *;