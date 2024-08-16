-- name: DeployInstructionsByID :one
SELECT * FROM deploy_instructions WHERE id = @id;

-- name: DeployInstructionsCreate :one
INSERT INTO deploy_instructions (
  environment_id,
  feature_name,
  feature_version,
  hash,
  values
) VALUES (
  @environment_id,
  @feature_name,
  @feature_version,
  @hash,
  @values
)
RETURNING id
;

-- name: DeployInstructionsUpdateStatus :exec
UPDATE deploy_instructions
SET status = @status
WHERE id = @id
;

-- name: TimeoutDeployInstructions :exec
UPDATE deploy_instructions
SET status = 'failed'
WHERE status = 'pending'
AND last_modified < NOW() - INTERVAL '1 hour'
;

-- name: DeployInstructionsLatestForFeature :one
SELECT * FROM deploy_instructions
WHERE feature_name = @feature_name
AND environment_id = @environment_id
ORDER BY created DESC
LIMIT 1
;

-- name: DeployInstructionsLatestForEnvironment :many
SELECT * FROM deploy_instructions
WHERE id IN (
  SELECT DISTINCT ON (feature_name) id
  FROM deploy_instructions di
  WHERE di.environment_id = @environment_id
  ORDER BY feature_name, created DESC
)
;

-- name: DeployInstructionsForFeature :many
SELECT * FROM deploy_instructions
WHERE feature_name = @feature_name
AND environment_id = @environment_id
ORDER BY created DESC
LIMIT 10 OFFSET sqlc.arg('offset')
;

-- name: DeployInstructionsForNameVersion :one
SELECT * FROM deploy_instructions
WHERE feature_name = @feature_name
AND feature_version = @feature_version
;

-- name: DeployInstructionsPrevious :one
WITH current AS (
  SELECT di.*
  FROM deploy_instructions di
  WHERE di.id = @id
)
SELECT * FROM deploy_instructions
WHERE feature_name = (SELECT feature_name FROM current)
AND environment_id = (SELECT environment_id FROM current)
AND created < (SELECT created FROM current)
ORDER BY created DESC
LIMIT 1
;

-- name: NamesFromDeployInstruction :one
SELECT environments.name as environment_name,
  tenants.name as tenant_name
FROM deploy_instructions
JOIN environments ON deploy_instructions.environment_id = environments.id
JOIN tenants ON environments.tenant_id = tenants.id
WHERE deploy_instructions.id = @id
;
