-- +goose Up
CREATE TABLE configurations
(
    "id"             uuid                 DEFAULT uuid_generate_v4(),
    "environment_id" uuid,
    "key"            TEXT        NOT NULL,
    "value"          JSONB       NOT NULL,
    "description"    TEXT,
    "deleted"        BOOLEAN              DEFAULT false,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);