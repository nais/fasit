-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_rollout_status_on_failure() RETURNS trigger AS $$
BEGIN
    IF NEW.type = 'failed' THEN
        UPDATE rollouts
        SET status = 'failed'
        WHERE id = NEW.rollout_id AND status IN ('pending', '');
    END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER configurations_global_notify
    BEFORE INSERT
    ON rollout_events
    FOR EACH ROW
    EXECUTE PROCEDURE update_rollout_status_on_failure();
