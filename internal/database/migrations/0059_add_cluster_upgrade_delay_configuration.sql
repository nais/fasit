-- +goose Up
-- Add WAITING status to cluster upgrades enum
ALTER TYPE cluster_upgrades_status
ADD VALUE 'WAITING'
AFTER 'CREATED'
;

-- Add upgrade_delay_days column to tenants table
ALTER TABLE tenants
ADD COLUMN upgrade_delay_days INT NOT NULL DEFAULT 0
;

-- Add upgrade_delay_days column to environments table
ALTER TABLE environments
ADD COLUMN upgrade_delay_days INT NOT NULL DEFAULT 0
;

-- +goose Down
ALTER TABLE environments
DROP COLUMN upgrade_delay_days
;

ALTER TABLE tenants
DROP COLUMN upgrade_delay_days
;

-- Note: PostgreSQL does not support removing enum values directly.
-- If rollback is needed, a new enum type must be created without 'WAITING'
-- and the cluster_upgrades.status column must be altered to use the new type.
