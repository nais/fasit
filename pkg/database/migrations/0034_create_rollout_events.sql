-- +goose Up
CREATE TABLE rollout_events(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    rollout_id uuid NOT NULL,
    failure bool NOT NULL,
    message JSONB NOT NULL,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    FOREIGN KEY ("rollout_id") REFERENCES rollouts ("id") ON DELETE CASCADE
);