-- +goose Up
ALTER TABLE environments
	DROP CONSTRAINT IF EXISTS environments_maintenance_window_valid,
	DROP COLUMN auto_upgrade,
	DROP COLUMN upgrade_delay_days,
	DROP COLUMN maintenance_window;

ALTER TABLE tenants
	DROP COLUMN upgrade_delay_days;

-- +goose Down
ALTER TABLE environments
	ADD COLUMN "auto_upgrade" BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN "upgrade_delay_days" INT NOT NULL DEFAULT 0,
	ADD COLUMN "maintenance_window" JSONB,
	ADD CONSTRAINT environments_maintenance_window_valid CHECK (maintenance_window IS NULL OR (JSONB_TYPEOF(maintenance_window) = 'object' AND JSONB_EXISTS(maintenance_window, 'startTime') AND JSONB_TYPEOF(maintenance_window -> 'startTime') = 'string' AND (maintenance_window ->> 'startTime') ~ '^([01]\d|2[0-3]):[0-5]\d$' AND JSONB_EXISTS(maintenance_window, 'endTime') AND JSONB_TYPEOF(maintenance_window -> 'endTime') = 'string' AND (maintenance_window ->> 'endTime') ~ '^([01]\d|2[0-3]):[0-5]\d$' AND JSONB_EXISTS(maintenance_window, 'days') AND JSONB_TYPEOF(maintenance_window -> 'days') = 'array'));

ALTER TABLE tenants
	ADD COLUMN "upgrade_delay_days" INT NOT NULL DEFAULT 0;

