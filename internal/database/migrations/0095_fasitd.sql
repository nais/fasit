-- +goose Up
-- fasitd dry-run persistence. While naisd remains the canonical rollout path,
-- Fasit also dispatches matching commands to fasitd over a gRPC session and
-- records the lifecycle here, fully separate from deploy_log. fasitd health is
-- represented by the live session itself, so there is no health table.
-- One immutable row per command Fasit dispatched to fasitd, keyed by diid.
CREATE TABLE fasitd_commands(
	diid UUID PRIMARY KEY,
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature_assignment_id UUID NOT NULL REFERENCES feature_assignments(id) ON DELETE CASCADE,
	feature_name TEXT NOT NULL,
	feature_version TEXT NOT NULL,
	chart TEXT NOT NULL DEFAULT '',
	config_hash TEXT NOT NULL DEFAULT '',
	uninstall BOOLEAN NOT NULL DEFAULT FALSE,
	"values" JSONB NOT NULL DEFAULT '{}',
	created TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX fasitd_commands_env_feature_idx ON fasitd_commands(environment_id, feature_name, created DESC);

CREATE TRIGGER fasitd_commands_no_modify
	BEFORE DELETE OR UPDATE ON fasitd_commands
	FOR EACH ROW
	EXECUTE FUNCTION prevent_modify();

-- Append-only lifecycle log for a command. Statuses reuse the rollout vocabulary
-- (sent, installing, deployed, failed) plus 'undeliverable' when no session
-- exists. diid repeats across transition rows; latest row per diid is current.
CREATE TABLE fasitd_command_statuses(
	id BIGSERIAL PRIMARY KEY,
	diid UUID NOT NULL REFERENCES fasitd_commands(diid) ON DELETE CASCADE,
	status TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	created TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX fasitd_command_statuses_diid_idx ON fasitd_command_statuses(diid, created DESC);

CREATE TRIGGER fasitd_command_statuses_no_modify
	BEFORE DELETE OR UPDATE ON fasitd_command_statuses
	FOR EACH ROW
	EXECUTE FUNCTION prevent_modify();

-- Latest lifecycle status per command.
CREATE VIEW fasitd_command_status AS SELECT DISTINCT ON (s.diid)
	s.diid,
	c.environment_id,
	c.feature_assignment_id,
	c.feature_name,
	c.feature_version,
	s.status,
	s.message,
	s.created
FROM
	fasitd_command_statuses s
	JOIN fasitd_commands c ON c.diid = s.diid
ORDER BY
	s.diid,
	s.created DESC,
	s.id DESC;

-- Helm logs reported by fasitd for a command.
CREATE TABLE fasitd_helm_logs(
	id BIGSERIAL PRIMARY KEY,
	diid UUID NOT NULL REFERENCES fasitd_commands(diid) ON DELETE CASCADE,
	time TIMESTAMPTZ NOT NULL,
	message TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT ''
);

CREATE INDEX fasitd_helm_logs_diid_idx ON fasitd_helm_logs(diid, time);

-- Latest helm release inventory reported by fasitd, per environment×feature.
CREATE TABLE fasitd_release_statuses(
	environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	feature TEXT NOT NULL,
	version TEXT NOT NULL,
	status TEXT NOT NULL,
	revision INTEGER NOT NULL,
	last_deployed TIMESTAMPTZ NOT NULL,
	created TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT fasitd_release_statuses_pkey PRIMARY KEY (environment_id, feature)
);

