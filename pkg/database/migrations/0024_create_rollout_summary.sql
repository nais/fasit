-- +goose Up
DELETE FROM rollouts;

CREATE TABLE rollout_summaries (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    "feature" TEXT NOT NULL,
    "created" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    "last_modified" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

ALTER TABLE rollouts
  ADD COLUMN kind environment_kind NOT NULL,
  ADD COLUMN rollout_summary_id UUID NOT NULL REFERENCES rollout_summaries (id)
    ON DELETE CASCADE
;
