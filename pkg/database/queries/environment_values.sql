-- name: EnvironmentValueStore :exec
INSERT INTO environment_values ("environment_id", "key", "value") VALUES (@envID, @key, @value)
ON CONFLICT ("environment_id", "key") DO UPDATE SET "value" = @value;

-- name: EnvironmentValueGet :one
SELECT * FROM environment_values WHERE "environment_id" = @envID AND "key" = @key;

-- name: EnvironmentValuesForEnvironment :one
WITH management_id AS (
  SELECT id
  FROM environments
  WHERE tenant_id = @tenantID
  AND kind = 'management'
),
management_values AS (
  SELECT
    json_object_agg("key", "value") AS management
  FROM environment_values, management_id
  WHERE environment_values.environment_id = management_id.id
),
environment_values AS (
  SELECT
    json_object_agg("key", "value") AS environment
  FROM environment_values
  WHERE environment_values.environment_id = @envID
)

SELECT
  COALESCE(management_values.management, '{}'::json),
  COALESCE(environment_values.environment, '{}'::json)
FROM management_values, environment_values;
