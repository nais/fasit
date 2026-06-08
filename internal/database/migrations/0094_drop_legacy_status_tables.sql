-- +goose Up
-- Cut over from the mutable status tables to the append-only logs (ADR 0003).
-- decision_log + deploy_log (migration 0093) now own reconciler decision and
-- deploy-lifecycle state, so the old tables are removed. logs keeps its
-- deploy_instruction column as a plain DIID correlation id; the FK to
-- deploy_instructions is dropped because that table is going away (and DIID now
-- repeats across deploy_log transition rows, so it cannot be a FK target).
ALTER TABLE logs
	DROP CONSTRAINT IF EXISTS logs_deploy_instruction_fkey;

DROP TABLE IF EXISTS deploy_instructions CASCADE;

DROP TABLE IF EXISTS feature_reconcile_statuses CASCADE;

