-- +goose Up
DROP VIEW IF EXISTS environment_values_stats;

DROP TABLE IF EXISTS environment_features;

DROP TABLE IF EXISTS features;

-- +goose Down
CREATE TABLE features(
	name TEXT PRIMARY KEY,
	version TEXT NOT NULL,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	FOREIGN KEY (NAME, version) REFERENCES feature_data(NAME, version)
);

CREATE TRIGGER features_set_modified
	BEFORE UPDATE ON features
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

CREATE TABLE environment_features(
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature_name TEXT NOT NULL,
	feature_version TEXT NOT NULL,
	deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
	PRIMARY KEY (environment_id, feature_name),
	FOREIGN KEY (feature_name, feature_version) REFERENCES feature_data(NAME, version) ON DELETE CASCADE
);

CREATE OR REPLACE VIEW environment_values_stats AS
WITH source AS (
	SELECT
		features.name AS feature_name,
		JSONB_ARRAY_ELEMENTS_TEXT((fd.tpl_details -> 'Env')::JSONB ||(fd.tpl_details -> 'Envs')::JSONB) AS key,
		fd.kinds AS kinds
	FROM
		feature_data fd
		INNER JOIN features ON features.name = fd.name
			AND features.version = fd.version
		UNION
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

