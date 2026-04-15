-- +goose Up
DELETE FROM audits
WHERE object_type = 'cluster_upgrades';

-- +goose Down
-- Data is not recoverable after deletion.
