-- +goose Up
ALTER TABLE cluster_upgrades
ADD COLUMN "is_automatic" BOOLEAN NOT NULL DEFAULT FALSE
;

-- +goose Down
ALTER TABLE cluster_upgrades
DROP COLUMN "is_automatic"
;
