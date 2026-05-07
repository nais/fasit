-- name: AuditCreate :exec
INSERT INTO audits(
	actor,
	description,
	object_type,
	object_id,
	metadata)
VALUES (
	@actor,
	@description,
	@object_type,
	@object_id,
	@metadata);

-- name: AuditForEnvironment :many
SELECT
	*
FROM
	audits
WHERE
	CASE WHEN @featureName::TEXT != '' THEN
		object_id = CONCAT(@environment_id::TEXT, ':', @featureName::TEXT)
	ELSE
		STARTS_WITH(object_id, @environment_id::TEXT)
	END
ORDER BY
	created_at DESC
LIMIT @page_size;

