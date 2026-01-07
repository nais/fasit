-- +goose Up
ALTER TABLE rollouts
	DROP CONSTRAINT rollouts_feature_name_key;

