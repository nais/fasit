-- +goose Up
-- +goose StatementBegin
-- Rename MASTER_UPGRADE to CONTROL_PLANE_UPGRADE in cluster_upgrades_status enum
ALTER TYPE cluster_upgrades_status
RENAME VALUE 'MASTER_UPGRADE' TO 'CONTROL_PLANE_UPGRADE'
;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TYPE cluster_upgrades_status
RENAME VALUE 'CONTROL_PLANE_UPGRADE' TO 'MASTER_UPGRADE'
;

-- +goose StatementEnd
