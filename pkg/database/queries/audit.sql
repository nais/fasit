-- name: AuditCreate :exec
INSERT INTO audits (
  actor,
  description,
  object_type,
  object_id
) VALUES (
  @actor,
  @description,
  @object_type,
  @object_id
);

-- name: AuditForEnvironment :many
SELECT *
FROM audits
WHERE CASE WHEN @featureName::text != '' THEN
  object_id = concat(@environment_id::text, ':', @featureName::text)
ELSE
  starts_with(object_id, @environment_id::text)
END
ORDER BY created_at DESC
LIMIT @page_size
;
