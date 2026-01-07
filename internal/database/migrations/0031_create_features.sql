-- +goose Up
CREATE TABLE features(
	name TEXT PRIMARY KEY,
	version TEXT NOT NULL,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	FOREIGN KEY (NAME, version) REFERENCES feature_data(NAME, version)
);

CREATE TRIGGER features_set_modified
	BEFORE UPDATE ON features
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

