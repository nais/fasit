-- +goose Up
ALTER TABLE deployments
	ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE deployments
	DROP COLUMN active;

