-- name: Get :one
SELECT
	*
FROM
	environments
WHERE
	id = @id;

-- name: ListCIEnvironmentsForTarget :many
SELECT DISTINCT
	sqlc.embed(e_ci),
	t.name AS tenant_name
FROM
	environments e_ci
	JOIN tenants t ON e_ci.tenant_id = t.id
WHERE
	e_ci.ci = TRUE
	AND EXISTS (
		SELECT
			1
		FROM
			environments e_non_ci
		WHERE
			e_non_ci.ci = FALSE
			AND e_non_ci.labels @> @target
			AND e_non_ci.kind = e_ci.kind)
ORDER BY
	e_ci.name ASC;

-- name: GetLabels :one
SELECT
	labels
FROM
	environments
WHERE
	id = @id;

