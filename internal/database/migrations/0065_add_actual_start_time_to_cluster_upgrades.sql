-- +goose Up
ALTER TABLE cluster_upgrades
ADD COLUMN upgrade_start_time TIMESTAMPTZ
;

-- +goose Down
ALTER TABLE cluster_upgrades
DROP COLUMN upgrade_start_time
;
