-- +goose Up
CREATE OR REPLACE VIEW environment_values_stats AS
WITH source AS (
	-- Keys using `.Env` and `.Envs`
	SELECT
		features.name AS feature_name,
		JSONB_ARRAY_ELEMENTS_TEXT((fd.tpl_details -> 'Env')::JSONB ||(fd.tpl_details -> 'Envs')::JSONB) AS key,
		fd.kinds AS kinds
	FROM
		feature_data fd
		INNER JOIN features ON features.name = fd.name
			AND features.version = fd.version
		UNION
		-- Keys using `.Management`
		SELECT
			features.name AS feature_name,
			JSONB_ARRAY_ELEMENTS_TEXT((fd.tpl_details -> 'Management')::JSONB) AS key,
			'{management}'::environment_kind[] AS kinds
		FROM
			feature_data fd
		INNER JOIN features ON features.name = fd.name
			AND features.version = fd.version
)
	SELECT
		key,
		kind,
		COUNT(key),
		ARRAY_AGG(feature_name) AS features
	FROM
		source,
		UNNEST(kinds) AS k(kind)
GROUP BY
	key,
	kind;

