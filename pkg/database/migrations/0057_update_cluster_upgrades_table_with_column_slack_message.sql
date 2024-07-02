-- +goose Up
ALTER TABLE cluster_upgrades
  ADD COLUMN "slack_message" TEXT;
ALTER TABLE cluster_upgrades
  ADD COLUMN "slack_channel_id" TEXT;
