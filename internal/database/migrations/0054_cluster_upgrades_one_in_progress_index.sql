-- +goose Up
CREATE UNIQUE INDEX cluster_upgrades_one_in_progress
ON cluster_upgrades (environment_id, (status in ('CREATED', 'MASTER_UPGRADE', 'NODE_UPGRADE')))
WHERE status in ('CREATED', 'MASTER_UPGRADE', 'NODE_UPGRADE');
