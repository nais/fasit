-- +goose Up
CREATE INDEX IF NOT EXISTS logs_deploy_instruction_time_idx
	ON logs(deploy_instruction, "time");
