-- +goose Up
CREATE TABLE rollout_events(
	"id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	"rollout_id" UUID NOT NULL,
	"type" TEXT NOT NULL,
	"data" JSONB NOT NULL,
	"created" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	FOREIGN KEY ("rollout_id") REFERENCES rollouts("id") ON DELETE CASCADE
);

