
-- For sqlc 1.15, when altering a parent table the inherited tables are not updated.
-- This is a workaround to make sure that the inherited tables are updated for the generated queries.
ALTER TABLE configurations_environment
  ADD COLUMN rollout_id UUID REFERENCES rollouts (id)
    ON DELETE SET NULL
;
