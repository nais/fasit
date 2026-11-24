-- +goose Up
ALTER TABLE fasitd_commands
	ALTER COLUMN feature_assignment_id DROP NOT NULL;

-- +goose Down
DELETE FROM fasitd_commands
WHERE feature_assignment_id IS NULL;

ALTER TABLE fasitd_commands
	ALTER COLUMN feature_assignment_id SET NOT NULL;

