-- +goose Up
CREATE TABLE features(
    name text PRIMARY KEY,
    version text NOT NULL,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "last_modified"  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    FOREIGN KEY (name, version) REFERENCES feature_data (name, version)
);

CREATE TRIGGER features_set_modified
    BEFORE UPDATE
    ON features
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();
