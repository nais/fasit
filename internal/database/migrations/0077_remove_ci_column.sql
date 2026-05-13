-- +goose Up
ALTER TABLE environments
	DROP COLUMN CI;

