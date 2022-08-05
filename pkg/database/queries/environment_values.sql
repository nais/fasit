-- name: EnvironmentValueStore :exec
INSERT INTO environment_values ("environment_id", "key", "value", "secret") VALUES (@envID, @key, @value, @secret)
ON CONFLICT ("environment_id", "key") DO UPDATE SET "value" = @value, "secret" = @secret;

-- name: EnvironmentValueGet :one
SELECT * FROM environment_values WHERE "environment_id" = @envID AND "key" = @key;

-- name: EnvironmentValuesForEnvironment :many
SELECT * FROM environment_values WHERE "environment_id" = @envID;

-- name: MappingValuesForTenant :many
SELECT
  "id",
  "name",
  "kind",
  -- FILTER is added to prevent error when there's no environment values
  coalesce(json_object_agg("key", "value") FILTER (WHERE "key" IS NOT NULL), '{}'::json)::json AS "values"
FROM environments
LEFT JOIN environment_values ON environment_values.environment_id = environments.id
WHERE tenant_id = @tenantID
GROUP BY "id", "name", "kind"
;
