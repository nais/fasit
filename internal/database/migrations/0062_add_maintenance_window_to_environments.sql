-- Add maintenance_window column to environments table
-- This stores the maintenance window as JSON for when automatic cluster upgrades are allowed
-- +goose Up
-- +goose StatementBegin
ALTER TABLE environments
ADD COLUMN maintenance_window JSONB,
ADD CONSTRAINT environments_maintenance_window_valid CHECK (
	maintenance_window IS NULL
	OR (
		JSONB_TYPEOF(maintenance_window) = 'object'
		AND maintenance_window ? 'day'
		AND maintenance_window ? 'start_time'
		AND maintenance_window ? 'end_time'
	)
)
;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE environments
DROP CONSTRAINT IF EXISTS environments_maintenance_window_valid,
DROP COLUMN maintenance_window
;

-- +goose StatementEnd
