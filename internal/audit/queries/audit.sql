-- name: AuditCreate :exec
INSERT INTO audits(
	actor,
	description,
	object_type,
	object_id)
VALUES (
	@actor,
	@description,
	@object_type,
	@object_id);

