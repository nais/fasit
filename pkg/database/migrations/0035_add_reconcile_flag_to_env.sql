-- +goose Up
ALTER TABLE environments ADD COLUMN reconcile bool NOT NULL DEFAULT true;

UPDATE environments SET reconcile = false;
