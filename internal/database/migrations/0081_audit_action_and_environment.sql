-- +goose Up
ALTER TABLE audits
	ADD COLUMN action TEXT NOT NULL DEFAULT '',
	ADD COLUMN environment_id UUID;

CREATE INDEX audits_environment_id_created_at_idx ON audits (environment_id, created_at DESC)
	WHERE environment_id IS NOT NULL;
