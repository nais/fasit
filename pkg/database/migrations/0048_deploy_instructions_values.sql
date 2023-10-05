-- +goose Up
ALTER TABLE deploy_instructions ADD COLUMN values JSONB NOT NULL DEFAULT '{}'::jsonb;
