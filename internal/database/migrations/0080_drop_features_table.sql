-- +goose Up
DROP VIEW IF EXISTS environment_values_stats;

DROP TABLE IF EXISTS environment_features;

DROP TABLE IF EXISTS features;

-- +goose Down
-- Irreversible: application code no longer uses these tables.

