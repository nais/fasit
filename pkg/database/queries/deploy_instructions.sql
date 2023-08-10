-- name: DeployInstructionsByID :one
SELECT * FROM deploy_instructions WHERE id = @id;

-- name: DeployInstructionsCreate :one
INSERT INTO deploy_instructions (
  environment_id,
  feature_name,
  feature_version,
  hash
) VALUES (
  @environment_id,
  @feature_name,
  @feature_version,
  @hash
)
RETURNING id
;

-- name: DeployInstructionsUpdateStatus :exec
UPDATE deploy_instructions
SET status = @status
WHERE id = @id
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
