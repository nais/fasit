-- +goose Up
ALTER TABLE rollouts
	ADD COLUMN deploy_instructions UUID[] NOT NULL DEFAULT '{}';

