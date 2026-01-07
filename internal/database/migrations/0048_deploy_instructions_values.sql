-- +goose Up
ALTER TABLE deploy_instructions
	ADD COLUMN
VALUES
	JSONB NOT NULL DEFAULT '{}'::JSONB;

