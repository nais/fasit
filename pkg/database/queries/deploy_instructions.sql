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
