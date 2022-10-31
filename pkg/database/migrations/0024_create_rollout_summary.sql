-- +goose Up
DELETE FROM rollouts;

CREATE TABLE rollout_summaries (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    "feature" TEXT NOT NULL,
    "created" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    "last_modified" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

ALTER TYPE rollout_status RENAME TO rollout_status_old;

CREATE TYPE rollout_status AS ENUM ('', 'pending', 'failed', 'deployed');

ALTER TABLE rollouts
  ALTER COLUMN status DROP DEFAULT,
  ALTER status TYPE rollout_status USING status::TEXT::rollout_status,
  ALTER COLUMN status SET DEFAULT ''::rollout_status,
  ADD COLUMN kind environment_kind NOT NULL,
  ADD COLUMN rollout_summary_id UUID NOT NULL REFERENCES rollout_summaries (id)
    ON DELETE CASCADE
;

DROP TYPE rollout_status_old;
