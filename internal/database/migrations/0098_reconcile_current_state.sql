-- +goose Up
-- deploy_status / decision_status used to compute the latest row per
-- environment x feature with DISTINCT ON over the whole append-only log, which
-- grows with history. deploy_current / decision_current hold that latest row,
-- maintained by AFTER INSERT triggers on the logs (triggers also fire for COPY,
-- which the reconciler uses). The views keep their shape so callers are
-- unchanged. The logs remain the source of truth; the backfill below can be
-- re-run to rebuild the current tables.
LOCK TABLE deploy_log,
decision_log IN SHARE ROW EXCLUSIVE MODE;

CREATE TABLE deploy_current(
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature_name TEXT NOT NULL,
	log_id BIGINT NOT NULL,
	diid UUID NOT NULL,
	feature_assignment_id UUID NOT NULL REFERENCES feature_assignments(id) ON DELETE CASCADE,
	feature_version TEXT NOT NULL,
	status TEXT NOT NULL,
	hash TEXT NOT NULL,
	created TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (environment_id, feature_name)
);

CREATE INDEX deploy_current_assignment_idx ON deploy_current(feature_assignment_id);

CREATE TABLE decision_current(
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature_name TEXT NOT NULL,
	log_id BIGINT NOT NULL,
	feature_assignment_id UUID NOT NULL REFERENCES feature_assignments(id) ON DELETE CASCADE,
	feature_version TEXT NOT NULL,
	action TEXT NOT NULL,
	message TEXT NOT NULL,
	created TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (environment_id, feature_name)
);

CREATE INDEX decision_current_assignment_idx ON decision_current(feature_assignment_id);

-- The WHERE guard keeps the views' "latest created DESC, id DESC wins" order
-- even if concurrent transactions commit out of order.
-- +goose StatementBegin
CREATE FUNCTION deploy_log_to_current()
	RETURNS TRIGGER
	LANGUAGE plpgsql
	AS $$
BEGIN
	INSERT INTO deploy_current AS c(
		environment_id,
		feature_name,
		log_id,
		diid,
		feature_assignment_id,
		feature_version,
		status,
		hash,
		created)
	VALUES(
		NEW.environment_id,
		NEW.feature_name,
		NEW.id,
		NEW.diid,
		NEW.feature_assignment_id,
		NEW.feature_version,
		NEW.status,
		NEW.hash,
		NEW.created)
ON CONFLICT(
	environment_id,
	feature_name)
	DO UPDATE SET
		log_id = EXCLUDED.log_id,
		diid = EXCLUDED.diid,
		feature_assignment_id = EXCLUDED.feature_assignment_id,
		feature_version = EXCLUDED.feature_version,
		status = EXCLUDED.status,
		hash = EXCLUDED.hash,
		created = EXCLUDED.created
	WHERE(
		EXCLUDED.created, EXCLUDED.log_id) >(
		c.created, c.log_id);
	RETURN NULL;
END;
$$;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION decision_log_to_current()
	RETURNS TRIGGER
	LANGUAGE plpgsql
	AS $$
BEGIN
	INSERT INTO decision_current AS c(
		environment_id,
		feature_name,
		log_id,
		feature_assignment_id,
		feature_version,
		action,
		message,
		created)
	VALUES(
		NEW.environment_id,
		NEW.feature_name,
		NEW.id,
		NEW.feature_assignment_id,
		NEW.feature_version,
		NEW.action,
		NEW.message,
		NEW.created)
ON CONFLICT(
	environment_id,
	feature_name)
	DO UPDATE SET
		log_id = EXCLUDED.log_id,
		feature_assignment_id = EXCLUDED.feature_assignment_id,
		feature_version = EXCLUDED.feature_version,
		action = EXCLUDED.action,
		message = EXCLUDED.message,
		created = EXCLUDED.created
	WHERE(
		EXCLUDED.created, EXCLUDED.log_id) >(
		c.created, c.log_id);
	RETURN NULL;
END;
$$;

-- +goose StatementEnd
INSERT INTO deploy_current(
	environment_id,
	feature_name,
	log_id,
	diid,
	feature_assignment_id,
	feature_version,
	status,
	hash,
	created)
SELECT
	environment_id,
	feature_name,
	id,
	diid,
	feature_assignment_id,
	feature_version,
	status,
	hash,
	created
FROM
	deploy_status;

INSERT INTO decision_current(
	environment_id,
	feature_name,
	log_id,
	feature_assignment_id,
	feature_version,
	action,
	message,
	created)
SELECT
	environment_id,
	feature_name,
	id,
	feature_assignment_id,
	feature_version,
	action,
	message,
	created
FROM
	decision_status;

CREATE TRIGGER deploy_log_current
	AFTER INSERT ON deploy_log
	FOR EACH ROW
	EXECUTE FUNCTION deploy_log_to_current();

CREATE TRIGGER decision_log_current
	AFTER INSERT ON decision_log
	FOR EACH ROW
	EXECUTE FUNCTION decision_log_to_current();

CREATE OR REPLACE VIEW decision_status AS
SELECT
	log_id AS id,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	action,
	message,
	created
FROM
	decision_current;

CREATE OR REPLACE VIEW deploy_status AS
SELECT
	log_id AS id,
	diid,
	environment_id,
	feature_assignment_id,
	feature_name,
	feature_version,
	status,
	hash,
	created
FROM
	deploy_current;

