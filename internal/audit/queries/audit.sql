-- name: Create :exec
INSERT INTO audits(
	actor,
	action,
	description,
	object_type,
	object_id,
	environment_id,
	metadata)
VALUES (
	@actor,
	@action,
	@description,
	@object_type,
	@object_id,
	@environment_id,
	@metadata);

-- name: List :many
SELECT
	a.*,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	audits a
LEFT JOIN environments e ON e.id = a.environment_id
LEFT JOIN tenants t ON t.id = e.tenant_id
WHERE
	a.environment_id = @environment_id::UUID
	AND (
		@feature_name::TEXT = ''
		OR a.object_id = @feature_name::TEXT
		OR STARTS_WITH(a.object_id, CONCAT(@feature_name::TEXT, '/'))
	)
ORDER BY
	a.created_at DESC
LIMIT @page_size;

-- name: ListForEnvironment :many
SELECT
	a.*,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	audits a
LEFT JOIN environments e ON e.id = a.environment_id
LEFT JOIN tenants t ON t.id = e.tenant_id
WHERE
	a.environment_id = @env_id
ORDER BY
	a.created_at DESC
LIMIT @page_size;

-- name: ListRecent :many
SELECT
	a.*,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	audits a
LEFT JOIN environments e ON e.id = a.environment_id
LEFT JOIN tenants t ON t.id = e.tenant_id
ORDER BY
	a.created_at DESC
LIMIT @page_size;
