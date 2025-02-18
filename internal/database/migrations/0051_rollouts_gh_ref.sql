-- +goose Up
DROP TRIGGER IF EXISTS rollouts_notify ON rollouts;

CREATE TRIGGER rollouts_notify
  AFTER INSERT OR UPDATE
  ON rollouts
  FOR EACH ROW
  EXECUTE PROCEDURE fasit_notify("id", "feature_name", "status");

ALTER TABLE rollouts ADD COLUMN gh_ref JSONB;
