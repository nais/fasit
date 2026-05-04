-- +goose Up
ALTER TABLE deployments
	DROP COLUMN ci;

