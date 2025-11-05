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

-- name: EnvironmentsTargetedByDeployment :many
SELECT
	el.environment_id
FROM
	deployments d
	JOIN environment_labels el ON TRUE
WHERE
	d.id = @deployment_id
	AND (d.target ->> el.key) = el.value
GROUP BY
	el.environment_id,
	d.target
HAVING
	COUNT(*) = (
		SELECT
			COUNT(*)
		FROM
			JSONB_OBJECT_KEYS(d.target)
	)
ORDER BY
	el.environment_id
;
