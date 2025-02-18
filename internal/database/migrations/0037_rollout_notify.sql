-- +goose Up
CREATE TRIGGER rollouts_notify
AFTER INSERT
ON rollouts
FOR EACH ROW
EXECUTE PROCEDURE configurations_notify_trigger();
