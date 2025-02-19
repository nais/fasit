-- +goose Up
CREATE TABLE feature_states (
	"environment_id" uuid NOT NULL,
	"feature" TEXT NOT NULL,
	"enabled" BOOLEAN NOT NULL DEFAULT FALSE,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (environment_id, feature),
	CONSTRAINT fk_feature_state_environment FOREIGN KEY (environment_id) REFERENCES environments (id) ON DELETE CASCADE
)
;

CREATE TRIGGER feature_states_set_modified BEFORE
UPDATE ON feature_states FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp ()
;
