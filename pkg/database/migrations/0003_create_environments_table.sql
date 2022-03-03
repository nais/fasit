-- +goose Up
CREATE TABLE environments
(
    "id"            uuid                 DEFAULT uuid_generate_v4(),
    "partner_id"    uuid        NOT NULL,
    "name"          TEXT        NOT NULL,
    "description"   TEXT,
    "created"       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
        CONSTRAINT fk_environments_partners
        FOREIGN KEY (partner_id)
        REFERENCES partners (id) ON DELETE CASCADE

);
CREATE TRIGGER environments_set_modified
    BEFORE UPDATE
    ON environments
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();

