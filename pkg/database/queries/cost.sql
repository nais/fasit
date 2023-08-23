-- name: CostUpsert :batchexec
INSERT INTO env_cost (
  tenant_id,
  env_id,
  date,
  cost
) VALUES (
  $1,
  $2,
  $3,
  $4
) ON CONFLICT (tenant_id, env_id, date) DO UPDATE SET
  cost = EXCLUDED.cost
;

-- name: CostLastDate :one
SELECT MAX(date)::DATE AS "date"
FROM env_cost
;

-- name: CostByTenant :many
SELECT
  tenant_id,
  "date",
  SUM(cost)::REAL AS cost
FROM env_cost
WHERE
  (
    tenant_id = sqlc.narg('tenant_id') OR
    sqlc.narg('tenant_id') IS NULL
  )
  AND "date" >= @start_date
  AND "date" <= @end_date
GROUP BY "date", tenant_id
ORDER BY "date", tenant_id
;
