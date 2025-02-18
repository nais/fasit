-- +goose Up
ALTER TABLE rollouts ADD COLUMN deploy_instructions uuid[] NOT NULL DEFAULT '{}';
