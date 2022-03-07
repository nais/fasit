-- name: ReconcileData :many
SELECT e.*, p.name AS partner_name
FROM environments e
JOIN partners p ON e.partner_id = p.id;
