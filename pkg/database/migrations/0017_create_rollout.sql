-- +goose Up
CREATE TYPE rollout_status AS ENUM ('', 'pending', 'deployed', 'failed');

CREATE TABLE rollouts (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    "feature" TEXT NOT NULL,
    "status" rollout_status NOT NULL DEFAULT '',
    "changeset" JSONB NOT NULL,
    "created" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    "last_modified" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TRIGGER rollouts_set_modified
    BEFORE UPDATE
    ON rollouts
    FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp();

-- The following is a trigger that will perform a notify on the rollout_notify channel
-- with the id of the rollout that was inserted. This is used to notify the rollout
-- service that a rollout has been created.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rollout_notify_trigger() RETURNS trigger AS $$
  BEGIN
    PERFORM pg_notify('rollout_notify', NEW.id::text);
    RETURN NULL;
  END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER rollouts_notify
    AFTER INSERT
    ON rollouts
    FOR EACH ROW
EXECUTE PROCEDURE rollout_notify_trigger();
