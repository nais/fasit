-- name: InsertEnvironmentFeature :exec
INSERT INTO environment_features(
	environment_id,
	feature_name,
	feature_version,
	deployment_id)
VALUES (
	@environment_id,
	@feature_name,
	@feature_version,
	@deployment_id)
ON CONFLICT (
	environment_id,
	feature_name)
	DO UPDATE SET
		feature_version = EXCLUDED.feature_version,
		deployment_id = EXCLUDED.deployment_id;

