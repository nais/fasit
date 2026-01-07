-- +goose Up
CREATE TABLE logs(
	id BIGSERIAL PRIMARY KEY NOT NULL,
	deploy_instruction UUID NOT NULL,
	TIME TIMESTAMPTZ NOT NULL,
	message TEXT NOT NULL,
	FOREIGN KEY (deploy_instruction) REFERENCES deploy_instructions(id) ON DELETE CASCADE
);

CREATE TRIGGER logs_notify
	AFTER INSERT ON logs
	FOR EACH ROW
	EXECUTE PROCEDURE fasit_notify("id", "deploy_instruction");

