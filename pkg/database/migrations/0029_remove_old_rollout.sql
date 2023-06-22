-- +goose Up

DROP TABLE rollout_events;
ALTER TABLE configurations_global
DROP COLUMN rollout_id;
DROP TABLE rollouts;
DROP TABLE rollout_summaries;
DROP TYPE rollout_status;
DROP FUNCTION update_rollout_status_on_failure();
DROP FUNCTION rollout_notify_trigger();
