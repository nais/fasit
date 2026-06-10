-- name: GetFeatureAssignment :one
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id::UUID;

-- name: ListTenants :many
SELECT
	*
FROM
	tenants
ORDER BY
	name;

-- name: ListTenantEnvironments :many
SELECT
	*
FROM
	environments
WHERE
	tenant_id = @tenant_id
ORDER BY
	name;

