-- +goose Up
ALTER TABLE "deployments"
	ADD COLUMN "ci" BOOL NOT NULL DEFAULT FALSE;

