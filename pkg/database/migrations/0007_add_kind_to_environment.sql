-- +goose Up
CREATE TYPE environment_kind AS ENUM ('partner', 'management');
ALTER TABLE environments ADD kind environment_kind DEFAULT 'partner' NOT NULL;
