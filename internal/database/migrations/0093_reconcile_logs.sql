-- +goose Up
-- Append-only changelog of reconciler compute decisions. A new row is inserted
-- only when the decision for a feature×environment actually changes; change
-- detection is done in Go (comparing against the latest row) for speed, not in
-- SQL. The compute_status view derives current state as the latest row per
-- feature×environment.
CREATE TABLE compute_changelog(
	id BIGSERIAL PRIMARY KEY,
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature_assignment_id UUID NOT NULL REFERENCES feature_assignments(id) ON DELETE CASCADE,
	feature_name TEXT NOT NULL,
	feature_version TEXT NOT NULL,
	action TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	created TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX compute_changelog_env_feature_idx ON compute_changelog(environment_id, feature_name, created DESC);

CREATE INDEX compute_changelog_assignment_idx ON compute_changelog(feature_assignment_id, created DESC);

CREATE TRIGGER compute_changelog_no_modify
	BEFORE DELETE OR UPDATE ON compute_changelog
	FOR EACH ROW
	EXECUTE FUNCTION prevent_modify();

-- Latest compute decision per feature×environment.
CREATE VIEW compute_status AS SELECT DISTINCT ON (environment_id, feature_name)
	id,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	action,
	message,
	created
FROM
	compute_changelog
ORDER BY
	environment_id,
	feature_name,
	created DESC;

-- Append-only log of deploy outcomes. Every deploy is a change, so every
-- dispatched deploy is recorded.
CREATE TABLE deploy_log(
	id BIGSERIAL PRIMARY KEY,
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature_assignment_id UUID NOT NULL REFERENCES feature_assignments(id) ON DELETE CASCADE,
	feature_name TEXT NOT NULL,
	feature_version TEXT NOT NULL,
	status TEXT NOT NULL,
	hash TEXT NOT NULL DEFAULT '',
	created TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX deploy_log_env_feature_idx ON deploy_log(environment_id, feature_name, created DESC);

CREATE INDEX deploy_log_assignment_idx ON deploy_log(feature_assignment_id, created DESC);

CREATE TRIGGER deploy_log_no_modify
	BEFORE DELETE OR UPDATE ON deploy_log
	FOR EACH ROW
	EXECUTE FUNCTION prevent_modify();

-- Latest deploy outcome per feature×environment.
CREATE VIEW deploy_status AS SELECT DISTINCT ON (environment_id, feature_name)
	id,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash,
	created
FROM
	deploy_log
ORDER BY
	environment_id,
	feature_name,
	created DESC;

