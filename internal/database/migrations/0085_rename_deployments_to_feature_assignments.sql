-- +goose Up
ALTER TABLE deployments RENAME TO feature_assignments;

ALTER TABLE deployment_statuses RENAME TO feature_reconcile_statuses;

ALTER TABLE feature_reconcile_statuses RENAME COLUMN deployment_id TO feature_assignment_id;

ALTER TABLE deploy_instructions RENAME COLUMN deployment_id TO feature_assignment_id;

ALTER INDEX deployments_pkey RENAME TO feature_assignments_pkey;

ALTER INDEX deployments_one_active_per_feature_target RENAME TO feature_assignments_one_active_per_feature_target;

ALTER INDEX deployment_statuses_pkey RENAME TO feature_reconcile_statuses_pkey;

ALTER INDEX deploy_instructions_deployment_id_idx RENAME TO deploy_instructions_feature_assignment_id_idx;

ALTER TRIGGER deployment_statuses_set_modified ON feature_reconcile_statuses RENAME TO feature_reconcile_statuses_set_modified;

