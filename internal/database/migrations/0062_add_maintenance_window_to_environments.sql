-- Add maintenance_window column to environments table
-- This stores the maintenance window as JSON for when automatic cluster upgrades are allowed
-- +goose Up
-- +goose StatementBegin
ALTER TABLE environments
ADD COLUMN maintenance_window JSONB
;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE environments
DROP COLUMN maintenance_window
;

-- +goose StatementEnd
