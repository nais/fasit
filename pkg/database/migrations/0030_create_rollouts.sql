-- +goose Up
CREATE TABLE rollouts(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    feature_name text NOT NULL UNIQUE,
    chart text NOT NULL UNIQUE,
    version text NOT NULL,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
