-- name: CostUpsert :batchexec
INSERT INTO
	env_cost (tenant_id, env_id, date, cost)
VALUES
	($1, $2, $3, $4)
ON CONFLICT (tenant_id, env_id, date) DO UPDATE
SET
	cost = EXCLUDED.cost
;

-- name: CostLastDate :one
SELECT
	MAX(date)::DATE AS "date"
FROM
	env_cost
;

-- name: CostForTenant :many
WITH
	envs AS (
		SELECT
			id
		FROM
			environments
		WHERE
			environments.tenant_id = @tenant_id
	),
	datasource AS (
		SELECT
			t.id AS env_id,
			t.tdate::DATE AS "date",
			COALESCE(SUM(cost)::REAL, 0.0) AS cost
		FROM
			(
				SELECT
					"day" AS "tdate",
					id AS "id"
				FROM
					GENERATE_SERIES(
						@start_date::DATE,
						@end_date::DATE,
						INTERVAL '1 day'
					) AS t (DAY),
					envs
			) AS t
			LEFT JOIN env_cost ON env_id = t.id
			AND "date"::DATE = t.tdate
		GROUP BY
			t.tdate,
			t.id
		ORDER BY
			t.tdate,
			t.id
	)
SELECT
	env_id,
	ARRAY_AGG(cost)::REAL[] AS cost
FROM
	datasource
GROUP BY
	env_id
ORDER BY
	env_id
;

-- name: Cost :many
WITH
	tenant_ids AS (
		SELECT
			id
		FROM
			tenants
	),
	datasource AS (
		SELECT
			t.id AS tenant_id,
			t.tdate::DATE AS "date",
			COALESCE(SUM(cost)::REAL, 0.0) AS cost
		FROM
			(
				SELECT
					"day" AS "tdate",
					id AS "id"
				FROM
					GENERATE_SERIES(
						@start_date::DATE,
						@end_date::DATE,
						INTERVAL '1 day'
					) AS t (DAY),
					tenant_ids
			) AS t
			LEFT JOIN env_cost ON tenant_id = t.id
			AND "date"::DATE = t.tdate
		GROUP BY
			t.tdate,
			t.id
		ORDER BY
			t.tdate,
			t.id
	)
SELECT
	tenant_id,
	ARRAY_AGG(cost)::REAL[] AS cost
FROM
	datasource
GROUP BY
	tenant_id
ORDER BY
	tenant_id
;
