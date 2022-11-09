-- name: Warnings :many
SELECT 'feature_status' as "type", environment_id, environment.tenant_id, feature
FROM status
JOIN environments environment ON environment.id = status.environment_id
WHERE status = 'failed'
AND (
  environment.id = @environment_id
  OR environment.tenant_id = @tenant_id
)

UNION

SELECT 'naisd', environment.id, environment.tenant_id, ''
FROM health_statuses hs
RIGHT JOIN environments environment ON environment.id = hs.environment_id
WHERE (
  hs.reported_at IS NULL
  OR hs.reported_at < NOW() - INTERVAL '10 minutes'
)
AND (
  environment.id = @environment_id
  OR environment.tenant_id = @tenant_id
)
;
