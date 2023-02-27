-- +goose Up
CREATE TABLE autoinstall(
    kind environment_kind NOT NULL,
    feature text NOT NULL REFERENCES features(name) ON DELETE CASCADE,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
