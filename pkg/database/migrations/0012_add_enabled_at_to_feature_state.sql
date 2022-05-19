-- +goose Up
ALTER TABLE feature_states ADD COLUMN "enabled_at" TIMESTAMPTZ;
