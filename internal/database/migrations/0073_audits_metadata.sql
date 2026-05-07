-- +goose Up
ALTER TABLE audits
	ADD COLUMN metadata JSONB;

