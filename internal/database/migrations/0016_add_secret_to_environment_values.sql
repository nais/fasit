-- +goose Up
ALTER TABLE environment_values
	ADD COLUMN "secret" BOOLEAN NOT NULL DEFAULT FALSE;

