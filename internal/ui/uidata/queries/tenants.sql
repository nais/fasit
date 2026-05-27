-- name: ListTenants :many
SELECT
	*
FROM
	tenants
ORDER BY
	created DESC,
	name ASC;

