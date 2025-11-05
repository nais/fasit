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
		AND JSONB_EXISTS (maintenance_window, 'startTime')
		AND JSONB_TYPEOF(maintenance_window -> 'startTime') = 'string'
		AND (maintenance_window ->> 'startTime') ~ '^\d{2}:\d{2}$'
		AND JSONB_EXISTS (maintenance_window, 'endTime')
		AND JSONB_TYPEOF(maintenance_window -> 'endTime') = 'string'
		AND (maintenance_window ->> 'endTime') ~ '^\d{2}:\d{2}$'
		AND JSONB_EXISTS (maintenance_window, 'days')
		AND JSONB_TYPEOF(maintenance_window -> 'days') = 'array'
		AND JSONB_EXISTS (maintenance_window, 'timezone')
		AND JSONB_TYPEOF(maintenance_window -> 'timezone') = 'string'
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
