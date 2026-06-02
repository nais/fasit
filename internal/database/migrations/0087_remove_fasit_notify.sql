-- +goose Up
DROP TRIGGER IF EXISTS configurations_global_notify ON configurations_global;
DROP TRIGGER IF EXISTS configurations_environment_notify ON configurations_environment;
DROP TRIGGER IF EXISTS deploy_instructions_notify ON deploy_instructions;
DROP TRIGGER IF EXISTS logs_notify ON logs;
DROP FUNCTION IF EXISTS fasit_notify();
