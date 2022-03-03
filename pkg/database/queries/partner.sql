-- name: GetPartner :one
SELECT *
FROM partners
WHERE id = @id;

-- name: GetPartners :many
SELECT *
FROM partners;
