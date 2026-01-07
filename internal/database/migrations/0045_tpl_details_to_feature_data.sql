-- +goose Up
ALTER TABLE feature_data
	ADD COLUMN tpl_details JSONB DEFAULT '{}'::JSONB;

