-- +goose Up
DROP INDEX IF EXISTS deploy_instructions_latest_idx;

CREATE INDEX IF NOT EXISTS deploy_instructions_latest_idx ON deploy_instructions(feature_name, environment_id, created DESC) INCLUDE (id, hash, status, feature_assignment_id);

