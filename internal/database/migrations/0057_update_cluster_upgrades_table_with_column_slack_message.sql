-- +goose Up
ALTER TABLE cluster_upgrades
	ADD COLUMN "slack_message_timestamp" TEXT;

ALTER TABLE cluster_upgrades
	ADD COLUMN "slack_channel_id" TEXT;

