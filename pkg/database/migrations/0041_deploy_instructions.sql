-- +goose Up

CREATE TABLE deploy_instructions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id UUID NOT NULL REFERENCES environments(id),
  feature_name TEXT NOT NULL,
  feature_version TEXT NOT NULL,
  hash TEXT NOT NULL
);

CREATE INDEX deploy_instructions_idx
ON deploy_instructions(
  feature_name,
  feature_version,
  environment_id
);
