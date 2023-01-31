-- +goose Up
CREATE TABLE incoming_feature(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name text NOT NULL UNIQUE,
    version text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);
