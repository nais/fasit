-- +goose Up
DROP INDEX IF EXISTS cluster_upgrades_one_in_progress;

CREATE UNIQUE INDEX cluster_upgrades_one_in_progress ON cluster_upgrades(environment_id)
WHERE
	status IN ('CREATED', 'WAITING', 'CONTROL_PLANE_UPGRADE', 'NODE_UPGRADE');

-- +goose Down
ALTER TABLE cluster_upgrades
	DROP CONSTRAINT IF EXISTS cluster_upgrades_one_in_progress;

CREATE UNIQUE INDEX cluster_upgrades_one_in_progress ON cluster_upgrades(environment_id)
WHERE
	status IN ('CREATED', 'MASTER_UPGRADE', 'NODE_UPGRADE');

