-- +goose Up
-- Add WAITING status to cluster upgrades enum
ALTER TYPE cluster_upgrades_status
	ADD VALUE 'WAITING' AFTER 'CREATED';

-- Add upgrade_delay_days column to tenants table
ALTER TABLE tenants
	ADD COLUMN upgrade_delay_days INT NOT NULL DEFAULT 0;

-- Add upgrade_delay_days column to environments table
ALTER TABLE environments
	ADD COLUMN upgrade_delay_days INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE environments
	DROP COLUMN upgrade_delay_days;

ALTER TABLE tenants
	DROP COLUMN upgrade_delay_days;

-- Note: PostgreSQL does not support removing enum values directly.
-- To rollback the addition of 'WAITING' to cluster_upgrades_status, perform the following steps manually:
-- 1. Create a new enum type without the 'WAITING' value:
--    CREATE TYPE cluster_upgrades_status_new AS ENUM ('CREATED', 'CONTROL_PLANE_UPGRADE', 'NODE_UPGRADE', 'FAILED', 'DONE');
-- 2. Alter the cluster_upgrades.status column to use the new type:
--    ALTER TABLE cluster_upgrades ALTER COLUMN status TYPE cluster_upgrades_status_new USING status::TEXT::cluster_upgrades_status_new;
-- 3. Drop the old enum type:
--    DROP TYPE cluster_upgrades_status;
-- 4. Rename the new type to the original name:
--    ALTER TYPE cluster_upgrades_status_new RENAME TO cluster_upgrades_status;
-- Manual intervention is required for this rollback.
