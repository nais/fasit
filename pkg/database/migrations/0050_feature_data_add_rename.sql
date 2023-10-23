-- +goose Up
ALTER TABLE feature_data ADD COLUMN rename JSONB NOT NULL DEFAULT '[]'::jsonb;
