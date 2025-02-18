-- +goose Up
ALTER TABLE status ADD COLUMN log text NOT NULL DEFAULT '';
