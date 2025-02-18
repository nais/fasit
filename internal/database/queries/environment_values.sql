-- name: EnvironmentValueStore :exec
INSERT INTO environment_values ("environment_id", "key", "value", "secret") VALUES (@envID, @key, @value, @secret)
ON CONFLICT ("environment_id", "key") DO UPDATE SET "value" = @value, "secret" = @secret;

-- name: EnvironmentValueGet :one
SELECT
"environment_id",
"key",
"secret",
(CASE WHEN secret THEN CASE WHEN @showSensitive::bool THEN value ELSE '"*****"' END ELSE value END)::jsonb AS "value"
FROM environment_values WHERE "environment_id" = @envID AND "key" = @key;

-- name: EnvironmentValuesForEnvironment :many
SELECT
"environment_id",
"environment_values"."key",
"secret",
(CASE WHEN secret THEN CASE WHEN @showSensitive::bool THEN value ELSE '"*****"' END ELSE value END)::jsonb AS "value",
COALESCE("evs"."count", 0) AS "count"
FROM environment_values
LEFT JOIN environments ON environments.id = environment_values.environment_id
LEFT JOIN environment_values_stats evs ON evs.key = environment_values.key AND evs.kind = environments.kind
WHERE "environment_id" = @envID
ORDER BY "environment_values"."key" ASC
;

-- name: MappingValuesForTenant :many
WITH environment_ids AS (
  SELECT id FROM environments WHERE "tenant_id" = @tenantID
),
mappings AS (
  SELECT
    "environment_id",
    "key",
    (CASE WHEN secret THEN CASE WHEN @showSensitive::bool THEN value ELSE '"*****"' END ELSE value END)::jsonb AS "value",
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
ORDER BY "name" ASC
;

-- name: EnvironmentValuesAcrossEnvs :many
SELECT
  ev.environment_id,
  ev.key,
  ev.secret,
  ev.value,
  t.id AS tenant_id,
  t.name AS tenant_name,
  e.name AS environment_name
FROM environment_values ev
JOIN environments e ON e.id = ev.environment_id
JOIN tenants t ON t.id = e.tenant_id
WHERE ev.key = @key
ORDER BY e.name ASC
;

-- name: EnvironmentValueDelete :exec
DELETE FROM environment_values WHERE "environment_id" = @envID AND "key" = @key;
