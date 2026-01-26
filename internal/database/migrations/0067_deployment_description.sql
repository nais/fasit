-- +goose Up
ALTER TABLE "deployments"
	ADD COLUMN "description" TEXT;

