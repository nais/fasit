-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION feature_states_trigger()
	RETURNS TRIGGER
	AS $$
BEGIN
	PERFORM
		pg_notify('feature_states_notify', NEW.environment_id::TEXT);
	RETURN NULL;
END;
$$
LANGUAGE plpgsql;

-- +goose StatementEnd
CREATE TRIGGER feature_states_notify
	AFTER INSERT OR UPDATE ON feature_states
	FOR EACH ROW
	EXECUTE PROCEDURE feature_states_trigger();

