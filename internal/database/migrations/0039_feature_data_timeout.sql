-- +goose Up
ALTER TABLE feature_data
	ADD COLUMN timeout BIGINT NOT NULL DEFAULT 0;

