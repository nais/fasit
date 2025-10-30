-- +goose Up
-- Add WAITING status to cluster upgrades enum
ALTER TYPE cluster_upgrades_status
ADD VALUE 'WAITING'
AFTER 'CREATED'
;

-- Add upgrade_delay_days column to tenants table
ALTER TABLE tenants
ADD COLUMN upgrade_delay_days INT NOT NULL DEFAULT 1
;

COMMENT ON COLUMN tenants.upgrade_delay_days IS 'Number of days to delay cluster upgrades for all environments in this tenant. 0=immediate (test/dev), 1=1 day (default/staging), 2=2 days (production). Delays are additive with environment-level delays.'
;

CREATE INDEX idx_tenants_upgrade_delay ON tenants (upgrade_delay_days)
;

-- Add upgrade_delay_days column to environments table
ALTER TABLE environments
ADD COLUMN upgrade_delay_days INT NOT NULL DEFAULT 1
;

COMMENT ON COLUMN environments.upgrade_delay_days IS 'Number of days to delay cluster upgrades for this environment. 0=immediate (test/dev), 1=1 day (default/staging), 2=2 days (production). Delays are additive with tenant-level delays. Total delay = tenant delay + environment delay.'
;

CREATE INDEX idx_environments_upgrade_delay ON environments (upgrade_delay_days)
;

-- +goose Down
DROP INDEX idx_environments_upgrade_delay
;

ALTER TABLE environments
DROP COLUMN upgrade_delay_days
;

DROP INDEX idx_tenants_upgrade_delay
;

ALTER TABLE tenants
DROP COLUMN upgrade_delay_days
;

-- Note: PostgreSQL does not support removing enum values directly.
-- If rollback is needed, a new enum type must be created without 'WAITING'
-- and the cluster_upgrades.status column must be altered to use the new type.
