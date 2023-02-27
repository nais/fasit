-- +goose Up
CREATE TABLE rollouts(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    feature_name text NOT NULL UNIQUE,
    version text NOT NULL,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    FOREIGN KEY (feature_name, version) REFERENCES feature_data (name, version)
);

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
