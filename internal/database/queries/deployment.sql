-- name: DeploymentsGet :many
SELECT
	*
FROM
	deployments
ORDER BY
	created ASC
;

-- name: DeploymentCreate :one
INSERT INTO
	deployments (feature_name, version, target, gh_ref, hash)
VALUES
	(@feature_name, @version, @target, @gh_ref, @hash)
RETURNING
	*
;

-- name: DeploymentTargetsGetAll :many
SELECT
	dt.*,
	e.name AS environment_name,
	t.name AS tenant_name,
	d.feature_name,
	d.version
FROM
	deployment_targets dt
	JOIN environments e ON e.id = dt.environment_id
	JOIN tenants t ON t.id = e.tenant_id
	JOIN deployments d ON d.id = dt.deployment_id
ORDER BY
	dt.created
;

-- name: DeploymentTargetsGet :many
SELECT
	*
FROM
	deployment_targets
WHERE
	deployment_id = @deployment_id
ORDER BY
	created
;

-- name: DeploymentTargetsGetPending :many
SELECT
	*
FROM
	deployment_targets
WHERE
	status = 'pending'
ORDER BY
	created ASC
;

-- name: DeploymentTargetsCreate :exec
INSERT INTO
	deployment_targets (deployment_id, environment_id)
VALUES
	(@deployment_id, @environment_id)
ON CONFLICT DO NOTHING
;

-- name: DeploymentTargetsUpdate :exec
UPDATE deployment_targets
SET
	status = @status,
	last_modified = NOW()
WHERE
	deployment_id = @deployment_id
	AND environment_id = @environment_id
;

-- name: FeatureDeploymentsForEnvironment :many
SELECT DISTINCT
	ON (d.feature_name, d.target) sqlc.embed(d),
	fd.name,
	fd.version,
	fd.chart,
	fd.description,
	fd.source,
	fd.kinds::TEXT[] AS kinds,
	fd.dependencies,
	fd.values,
	fd.default_values,
	fd.timeout
FROM
	deployments d,
	environments e,
	feature_data fd
WHERE
	d.feature_name = fd.name
	AND d.version = fd.version
	AND e.id = @environment_id
	AND e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
ORDER BY
	d.feature_name,
	d.target,
	d.created DESC
;

-- name: FeatureEnabled :one
SELECT
	NOT EXISTS (
		SELECT
			*
		FROM
			feature_states fs
		WHERE
			fs.feature = @feature_name
			AND fs.environment_id = @environment_id
			AND fs.enabled = FALSE
	)
;

-- name: DeployInstructionsGetDeployedFeatures :many
SELECT
	feature_name
FROM
	deploy_instructions
WHERE
	feature_name = ANY (@feature_names::TEXT[])
	AND status = 'deployed'
	AND environment_id = @environment_id
	AND deployment_id IS NOT NULL
ORDER BY
	feature_name
;
