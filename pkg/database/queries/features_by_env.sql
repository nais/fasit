-- name: FeaturesForEnvironment :many
WITH "all" AS (
  SELECT *, 0 as rollout
  FROM features

  UNION ALL

  SELECT *, null, 1 as rollout
  FROM rollouts
), "env" AS (
  SELECT kind, ci
  FROM environments
  WHERE environments.id = @environment_id
), "result" AS (
  SELECT *
  FROM "all"
  JOIN env ON env.kind = ANY("all".environment_kinds)
), "filtered" AS (
  SELECT *, RANK() OVER (
      PARTITION BY "name"
      ORDER BY
      CASE WHEN ci THEN -rollout ELSE rollout END ASC,
        "name" ASC
    )
  FROM "result"
)

SELECT id, name, chart, version, created, last_modified, rollout::bool
FROM filtered
WHERE rank = 1;
