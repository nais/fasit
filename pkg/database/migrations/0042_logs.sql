-- +goose Up

CREATE TABLE logs (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  deploy_instruction UUID NOT NULL REFERENCES deploy_instructions(id),
  time TIMESTAMPTZ NOT NULL,
  message TEXT NOT NULL
);

CREATE TRIGGER logs_notify
  AFTER INSERT
  ON logs
  FOR EACH ROW
  EXECUTE PROCEDURE fasit_notify("id", "deploy_instruction");
