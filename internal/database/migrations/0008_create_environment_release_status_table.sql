-- +goose Up
CREATE TABLE release_statuses(
	"environment_id" UUID,
	"feature" TEXT NOT NULL,
	"version" TEXT NOT NULL,
	"status" TEXT NOT NULL,
	"revision" INT NOT NULL,
	"last_deployed" TIMESTAMPTZ NOT NULL,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (environment_id, feature),
	CONSTRAINT fk_release_statuses_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TRIGGER release_statuses_set_modified
	BEFORE UPDATE ON release_statuses
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

