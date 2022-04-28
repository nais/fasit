-- +goose Up
CREATE TABLE tenants
(
    "id"            uuid                 DEFAULT uuid_generate_v4(),
    "name"          TEXT        NOT NULL,
    "description"   TEXT,
    "created"       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);
CREATE TRIGGER tenant_set_modified
    BEFORE UPDATE
    ON tenants
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();
