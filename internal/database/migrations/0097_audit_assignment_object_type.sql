-- +goose Up
-- Feature assignments were historically audited with the legacy object_type
-- "deployment" and a "deploymentId"/"deploymentID" metadata key. Rename both to
-- the "assignment" vocabulary used everywhere else. The audits table is
-- append-only (audits_no_modify), so the trigger is temporarily disabled to
-- backfill existing rows.
ALTER TABLE audits DISABLE TRIGGER audits_no_modify;

UPDATE
	audits
SET
	object_type = 'assignment'
WHERE
	object_type = 'deployment';

UPDATE
	audits
SET
	metadata =(metadata - 'deploymentId' - 'deploymentID') || jsonb_build_object('assignmentId', COALESCE(metadata -> 'deploymentId', metadata -> 'deploymentID'))
WHERE
	metadata ? 'deploymentId'
	OR metadata ? 'deploymentID';

ALTER TABLE audits ENABLE TRIGGER audits_no_modify;

-- +goose Down
ALTER TABLE audits DISABLE TRIGGER audits_no_modify;

UPDATE
	audits
SET
	object_type = 'deployment'
WHERE
	object_type = 'assignment';

UPDATE
	audits
SET
	metadata =(metadata - 'assignmentId') || jsonb_build_object('deploymentId', metadata -> 'assignmentId')
WHERE
	metadata ? 'assignmentId';

ALTER TABLE audits ENABLE TRIGGER audits_no_modify;

