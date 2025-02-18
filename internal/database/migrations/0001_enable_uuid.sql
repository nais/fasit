-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE OR REPLACE FUNCTION update_modified_timestamp() RETURNS TRIGGER AS
$$ BEGIN NEW.last_modified = NOW(); RETURN NEW; END; $$
    LANGUAGE plpgsql;
