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
		AND (maintenance_window ->> 'startTime') ~ '^([01]\d|2[0-3]):[0-5]\d$'
		AND JSONB_EXISTS (maintenance_window, 'endTime')
		AND JSONB_TYPEOF(maintenance_window -> 'endTime') = 'string'
		AND (maintenance_window ->> 'endTime') ~ '^([01]\d|2[0-3]):[0-5]\d$'
		AND JSONB_EXISTS (maintenance_window, 'days')
		AND JSONB_TYPEOF(maintenance_window -> 'days') = 'array'
		AND JSONB_EXISTS (maintenance_window, 'timezone')
		AND JSONB_TYPEOF(maintenance_window -> 'timezone') = 'string'
		AND (maintenance_window ->> 'timezone') ~ '^[A-Za-z]+(?:/[A-Za-z_\-\.]+)+$'
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
