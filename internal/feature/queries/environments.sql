-- original name: MappingValuesForTenant
-- name: ListMappingValuesForTenant :many
WITH environment_ids AS (
	SELECT
		id
	FROM
		environments
	WHERE
		"tenant_id" = @tenantID
),
mappings AS (
	SELECT
		"environment_id",
		"key",
(
			CASE WHEN secret THEN
				CASE WHEN @showSensitive::BOOL THEN
					value
				ELSE
					'"*****"'
				END
			ELSE
				value
			END)::JSONB AS "value",
		"secret"
	FROM
		environment_values
	WHERE
		"environment_id" IN (
			SELECT
				*
			FROM
				environment_ids))
SELECT
	"id",
	"name",
	"kind",
	-- FILTER is added to prevent error when there's no environment values
	COALESCE(JSON_OBJECT_AGG("key", "value") FILTER (WHERE "key" IS NOT NULL), '{}'::JSON)::JSON AS "values"
FROM
	environments
	LEFT JOIN mappings ON mappings.environment_id = environments.id
WHERE
	environments.tenant_id = @tenantID
GROUP BY
	"id",
	"name",
	"kind"
ORDER BY
	"name" ASC;

-- name: ListSecretKeysForTenant :many
SELECT
	"environment_id",
	"key"
FROM
	environment_values
WHERE
	"environment_id" IN (
		SELECT
			id
		FROM
			environments
		WHERE
			"tenant_id" = @tenantID)
	AND "secret" = TRUE
ORDER BY
	"environment_id",
	"key";

