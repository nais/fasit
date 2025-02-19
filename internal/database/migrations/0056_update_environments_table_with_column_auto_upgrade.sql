-- +goose Up
ALTER TABLE environments
ADD COLUMN "auto_upgrade" BOOLEAN NOT NULL DEFAULT FALSE
;
