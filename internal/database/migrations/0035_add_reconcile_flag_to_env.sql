-- +goose Up
ALTER TABLE environments
ADD COLUMN reconcile BOOL NOT NULL DEFAULT TRUE
;

UPDATE environments
SET
	reconcile = FALSE
;
