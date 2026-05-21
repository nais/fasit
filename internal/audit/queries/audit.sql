-- name: Create :exec
INSERT INTO audits(
	actor,
	action,
	description,
	object_type,
	object_id,
	feature,
	environment_id,
	metadata)
VALUES (
	@actor,
	@action,
	@description,
	@object_type,
	@object_id,
	@feature,
	@environment_id,
	@metadata);

-- name: ListForFeature :many
SELECT
	a.*,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	audits a
LEFT JOIN environments e ON e.id = a.environment_id
LEFT JOIN tenants t ON t.id = e.tenant_id
WHERE
	a.feature = @feature
ORDER BY
	a.created_at DESC
LIMIT @page_size;

-- name: ListForFeatureInEnvironment :many
SELECT
	a.*,
	e.name AS environment_name,
	t.name AS tenant_name
FROM
	audits a
LEFT JOIN environments e ON e.id = a.environment_id
LEFT JOIN tenants t ON t.id = e.tenant_id
WHERE
	a.feature = @feature
	AND a.environment_id = @env_id
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
