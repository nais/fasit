-- +goose Up
CREATE TABLE features(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name text NOT NULL UNIQUE,
    chart text NOT NULL UNIQUE,
    environment_kinds environment_kind[] NOT NULL,
    version text NOT NULL,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "last_modified"  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER features_set_modified
    BEFORE UPDATE
    ON features
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();
