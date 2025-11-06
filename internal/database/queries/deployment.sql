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

-- name: EnvironmentsForDeployment :many
SELECT
	e.id
FROM
	deployments d,
	environments e
WHERE
	d.id = @deployment_id
	AND e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
ORDER BY
	e.id
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
