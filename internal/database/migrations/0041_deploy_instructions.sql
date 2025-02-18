-- +goose Up
CREATE TABLE deploy_instructions (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
	environment_id UUID NOT NULL,
	feature_name TEXT NOT NULL,
	feature_version TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'created',
	hash TEXT NOT NULL,
	created TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_modified TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT fk_deploy_instructions_environments FOREIGN KEY (environment_id) REFERENCES environments (id) ON DELETE CASCADE
)
;

CREATE INDEX deploy_instructions_idx ON deploy_instructions (feature_name, feature_version, environment_id)
;

CREATE TRIGGER deploy_instructions_set_modified BEFORE
UPDATE ON deploy_instructions FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp ()
;

CREATE TRIGGER deploy_instructions_notify
AFTER INSERT
OR
UPDATE ON deploy_instructions FOR EACH ROW
EXECUTE PROCEDURE fasit_notify ("id")
;

-- Copy over old data
INSERT INTO
	deploy_instructions (
		environment_id,
		feature_name,
		feature_version,
		status,
		hash,
		created,
		last_modified
	)
SELECT
	environment_id,
	feature AS feature_name,
	version AS feature_version,
	status,
	config_hash AS hash,
	created,
	last_modified
FROM
	status
;

;
