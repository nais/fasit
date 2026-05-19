-- +goose Up
DROP TRIGGER IF EXISTS feature_states_notify ON feature_states;

DROP TRIGGER IF EXISTS feature_states_set_modified ON feature_states;

DROP TABLE IF EXISTS feature_states;

