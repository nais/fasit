-- +goose Up
ALTER TABLE feature_data ADD COLUMN moved JSONB NOT NULL DEFAULT '[]'::jsonb;
