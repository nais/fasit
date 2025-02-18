-- +goose Up
ALTER TABLE feature_data
DROP COLUMN timeout
;
