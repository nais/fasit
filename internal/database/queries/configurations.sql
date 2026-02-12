-- name: EnvConfigOnlyKnown :many
WITH "combined" AS (
	SELECT
		"id",
		"feature",
		"key",
		"value",
		NULL::UUID AS environment_id
	FROM
		ONLY configurations_global glob
	WHERE
		glob.feature = @feature
		AND glob.key = ANY (@includedKeys::TEXT[])
	UNION
	SELECT
		"id",
		"feature",
		"key",
		"value",
		"environment_id"
	FROM
		ONLY configurations_environment env
	WHERE
		env.feature = @feature
		AND environment_id = @environment_id
		AND env.key = ANY (@includedKeys::TEXT[])
),
"filtered" AS (
	SELECT
		*,
		RANK() OVER (PARTITION BY "key" ORDER BY environment_id ASC,
			key ASC)
	FROM "combined"
)
SELECT
	*
FROM
	filtered
WHERE
	RANK = 1;

