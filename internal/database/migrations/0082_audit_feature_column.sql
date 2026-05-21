-- +goose Up
ALTER TABLE audits ADD COLUMN feature TEXT NOT NULL DEFAULT '';

CREATE INDEX audits_feature_created_at_idx ON audits (feature, created_at DESC)
	WHERE feature != '';

CREATE INDEX audits_feature_environment_id_idx ON audits (feature, environment_id, created_at DESC)
	WHERE feature != '' AND environment_id IS NOT NULL;
