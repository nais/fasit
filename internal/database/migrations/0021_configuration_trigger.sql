-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION configurations_notify_trigger () RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('configurations_notify', NEW.id::text);
RETURN NULL;
END;
$$ LANGUAGE plpgsql
;

-- +goose StatementEnd
CREATE TRIGGER configurations_global_notify
AFTER INSERT
OR
UPDATE ON configurations_global FOR EACH ROW
EXECUTE PROCEDURE configurations_notify_trigger ()
;

CREATE TRIGGER configurations_environment_notify
AFTER INSERT
OR
UPDATE ON configurations_environment FOR EACH ROW
EXECUTE PROCEDURE configurations_notify_trigger ()
;
