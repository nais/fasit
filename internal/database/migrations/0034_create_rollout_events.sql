-- +goose Up
CREATE TABLE rollout_events(
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	rollout_id UUID NOT NULL,
	failure BOOL NOT NULL,
	message TEXT NOT NULL,
	data JSONB,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	FOREIGN KEY ("rollout_id") REFERENCES rollouts("id") ON DELETE CASCADE
);

