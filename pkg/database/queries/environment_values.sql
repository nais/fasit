-- name: EnvironmentValueStore :exec
INSERT INTO environment_values ("environment_id", "key", "value", "secret") VALUES (@envID, @key, @value, @secret)
ON CONFLICT ("environment_id", "key") DO UPDATE SET "value" = @value, "secret" = @secret;

-- name: EnvironmentValueGet :one
SELECT
"environment_id",
"key",
"secret",
(CASE WHEN secret THEN CASE WHEN @showSensitive::bool THEN value ELSE '"***"' END ELSE value END)::jsonb AS "value"
FROM environment_values WHERE "environment_id" = @envID AND "key" = @key;

-- name: EnvironmentValuesForEnvironment :many
SELECT
"environment_id",
"key",
"secret",
(CASE WHEN secret THEN CASE WHEN @showSensitive::bool THEN value ELSE '"***"' END ELSE value END)::jsonb AS "value"
FROM environment_values WHERE "environment_id" = @envID;

-- name: MappingValuesForTenant :many
WITH environment_ids AS (
  SELECT id FROM environments WHERE "tenant_id" = @tenantID
),
mappings AS (
  SELECT
    "environment_id",
    "key",
    (CASE WHEN secret THEN CASE WHEN @showSensitive::bool THEN value ELSE '"***"' END ELSE value END)::jsonb AS "value",
    "secret"
  FROM environment_values
  WHERE "environment_id" IN (SELECT * FROM environment_ids)
)
SELECT
  "id",
  "name",
  "kind",
  -- FILTER is added to prevent error when there's no environment values
  coalesce(json_object_agg("key", "value") FILTER (WHERE "key" IS NOT NULL), '{}'::json)::json AS "values"
FROM environments
LEFT JOIN mappings ON mappings.environment_id = environments.id
WHERE environments.tenant_id = @tenantID
GROUP BY "id", "name", "kind"
;
