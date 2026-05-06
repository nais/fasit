-- name: GetEnvironmentFeature :one
SELECT
	sqlc.embed(fd),
	ef.deployment_id
FROM
	environment_features ef
	JOIN feature_data fd ON fd.name = ef.feature_name
		AND fd.version = ef.feature_version
WHERE
	environment_id = @environment_id
	AND feature_name = @feature_name;

-- name: HasMatchingDeployment :one
-- Returns TRUE when at least one deployment exists whose target labels
-- are contained by the environment's labels (matching the predicate used
-- by the deployment reconciler in ListDeploymentsToReconcile).
SELECT
	EXISTS (
		SELECT
			1
		FROM
			deployments d
			JOIN environments e ON e.id = @environment_id
				AND e.labels @> d.target
		WHERE
			d.feature_name = @feature_name);

