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
	deployments (feature_name, version, target, gh_ref)
VALUES
	(@feature_name, @version, @target, @gh_ref)
RETURNING
	*
;

-- name: DeploymentTargetsGetAll :many
SELECT
	*
FROM
	deployment_targets
ORDER BY
	created ASC
;

-- name: DeploymentTargetsGet :many
SELECT
	*
FROM
	deployment_targets
WHERE
	deployment_id = @deployment_id
ORDER BY
	created ASC
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
	deployment_targets (deployment_id, environment_id, hash)
VALUES
	(@deployment_id, @environment_id, @hash)
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

-- EnvironmentsForDeployment returns slice of env ids where feature is enabled and targeted by deployment
-- name: EnvironmentsForDeployment :many
SELECT
	e.id
FROM
	deployments d,
	environments e
	JOIN feature_states f ON f.feature_name = d.feature_name
	AND f.environment_id = e.id
WHERE
	d.id = @deployment_id
	AND f.enabled = TRUE
	AND e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
ORDER BY
	e.id
;
