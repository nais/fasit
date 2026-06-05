-- +goose Up
CREATE INDEX IF NOT EXISTS deploy_instructions_latest_idx ON deploy_instructions(feature_name, environment_id, created DESC);

CREATE INDEX IF NOT EXISTS deploy_instructions_deployed_idx ON deploy_instructions(feature_name, environment_id)
WHERE
	status = 'deployed';

