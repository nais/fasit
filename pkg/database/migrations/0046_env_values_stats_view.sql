-- +goose Up
CREATE OR REPLACE VIEW environment_values_stats AS
WITH source AS (
  SELECT
    DISTINCT JSONB_ARRAY_ELEMENTS_TEXT((fd.tpl_details->'Env')::jsonb || (fd.tpl_details->'Envs')::jsonb) AS key,
    fd.kinds AS kinds
  FROM feature_data fd
  INNER JOIN features ON features.name = fd.name AND features.version = fd.version
)
SELECT key, kind, count(key)
FROM source,
  UNNEST(kinds) AS k(kind)
GROUP BY key, kind
;
