-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rollout_trigger() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('rollout_notify', NEW.id::text);
RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER rollout_notify
    AFTER INSERT OR UPDATE
    ON rollouts
    FOR EACH ROW
    EXECUTE PROCEDURE rollout_trigger();
