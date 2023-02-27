-- +goose Up
CREATE TABLE auto_installs(
    kind environment_kind NOT NULL,
    feature text NOT NULL REFERENCES features(name) ON DELETE CASCADE,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
