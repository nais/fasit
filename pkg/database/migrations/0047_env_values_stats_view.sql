-- +goose Up
CREATE OR REPLACE VIEW environment_values_stats AS
WITH source AS (
  -- Keys using `.Env` and `.Envs`
  SELECT
    features.name AS feature_name,
    JSONB_ARRAY_ELEMENTS_TEXT((fd.tpl_details->'Env')::jsonb || (fd.tpl_details->'Envs')::jsonb) AS key,
    fd.kinds AS kinds
  FROM feature_data fd
  INNER JOIN features ON features.name = fd.name AND features.version = fd.version

  UNION

  -- Keys using `.Management`
  SELECT
    features.name AS feature_name,
    JSONB_ARRAY_ELEMENTS_TEXT((fd.tpl_details->'Management')::jsonb) AS key,
    '{management}'::environment_kind[] AS kinds
  FROM feature_data fd
  INNER JOIN features ON features.name = fd.name AND features.version = fd.version
)
SELECT key, kind, count(key), array_agg(feature_name) AS features
FROM source,
  UNNEST(kinds) AS k(kind)
GROUP BY key, kind
;
