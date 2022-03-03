-- +goose Up
CREATE TABLE configurations
(
    "id"             uuid                 DEFAULT uuid_generate_v4(),
    "environment_id" uuid,
    "feature"        TEXT        NOT NULL,
    "key"            TEXT        NOT NULL,
    "value"          JSONB       NOT NULL,
    "description"    TEXT,
    "secret"         BOOLEAN     NOT NULL DEFAULT false,
    "deleted"        BOOLEAN              DEFAULT false,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);