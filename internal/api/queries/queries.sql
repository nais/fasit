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

-- name: ListTenantsWithEnvironments :many
SELECT
	sqlc.embed(t),
	sqlc.embed(e),
	ev.value AS gcp_project_id
FROM
	tenants t
	JOIN environments e ON e.tenant_id = t.id
	LEFT JOIN environment_values ev ON ev.environment_id = e.id
		AND ev.key = 'project_id'
		AND ev.secret = FALSE
	ORDER BY
		t.name,
		e.name;

