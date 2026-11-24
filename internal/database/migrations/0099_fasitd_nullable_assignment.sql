-- +goose Up
ALTER TABLE fasitd_commands
	ALTER COLUMN feature_assignment_id DROP NOT NULL;

-- +goose Down
-- Uninstall commands have no assignment and must go before NOT NULL can be
-- restored. Both tables are append-only (prevent_modify), and the delete
-- cascades to fasitd_command_statuses, so the guards are lifted for the delete.
ALTER TABLE fasitd_commands DISABLE TRIGGER fasitd_commands_no_modify;

ALTER TABLE fasitd_command_statuses DISABLE TRIGGER fasitd_command_statuses_no_modify;

DELETE FROM fasitd_commands
WHERE feature_assignment_id IS NULL;

ALTER TABLE fasitd_command_statuses ENABLE TRIGGER fasitd_command_statuses_no_modify;

ALTER TABLE fasitd_commands ENABLE TRIGGER fasitd_commands_no_modify;

ALTER TABLE fasitd_commands
	ALTER COLUMN feature_assignment_id SET NOT NULL;

