-- +goose Up
ALTER TABLE configurations_global
ADD COLUMN rollout_id UUID REFERENCES rollouts (id) ON DELETE SET NULL
;
