-- name: ListTenantEnvironments :many
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
ORDER BY
	CASE WHEN name = 'management' THEN
		1
	ELSE
		2
	END,
	name ASC;

