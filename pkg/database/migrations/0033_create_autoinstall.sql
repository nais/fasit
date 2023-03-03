-- +goose Up
CREATE TABLE auto_installs(
    kind environment_kind NOT NULL,
    feature text NOT NULL REFERENCES features(name) ON DELETE CASCADE,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auto_installs_trigger() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('auto_installs_notify', NEW.kind::text, NEW.feature::text);
RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER auto_installs_notify
    AFTER INSERT OR UPDATE
    ON auto_installs
    FOR EACH ROW
    EXECUTE PROCEDURE auto_installs_trigger();
