-- name: Create :exec
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

-- name: List :many
SELECT
	*
FROM
	audits
WHERE
	CASE WHEN @feature_name::TEXT != '' THEN
		object_id = CONCAT(@environment_id::TEXT, ':', @feature_name::TEXT)
		OR (metadata IS NOT NULL
			AND metadata ->> 'feature' = @feature_name::TEXT
			AND (metadata ->> 'envId' = @environment_id::TEXT
				OR NOT (metadata ? 'envId')))
	ELSE
		STARTS_WITH(object_id, @environment_id::TEXT)
		OR (metadata IS NOT NULL
			AND metadata ->> 'envId' = @environment_id::TEXT)
	END
ORDER BY
	created_at DESC
LIMIT @page_size;

