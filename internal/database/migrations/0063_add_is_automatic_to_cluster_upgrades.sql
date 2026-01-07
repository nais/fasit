-- +goose Up
ALTER TABLE cluster_upgrades
	ADD COLUMN "is_automatic" BOOLEAN DEFAULT NULL;

-- +goose Down
ALTER TABLE cluster_upgrades
	DROP COLUMN "is_automatic";

