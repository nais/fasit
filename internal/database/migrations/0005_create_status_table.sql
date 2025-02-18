-- +goose Up
CREATE TABLE status (
	"environment_id" uuid,
	"feature" TEXT NOT NULL,
	"version" TEXT NOT NULL,
	"status" TEXT NOT NULL,
	"config_hash" TEXT NOT NULL,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (environment_id, feature),
	CONSTRAINT fk_status_environments FOREIGN KEY (environment_id) REFERENCES environments (id) ON DELETE CASCADE
)
;

CREATE TRIGGER status_set_modified BEFORE
UPDATE ON status FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp ()
;
