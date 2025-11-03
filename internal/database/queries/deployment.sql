-- name: DeploymentsGet :many
SELECT
	*
FROM
	deployment
ORDER BY
	created ASC
;

-- name: DeploymentTargetsGet :many
SELECT
	*
FROM
	deployment_targets
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
INSERT INTO deployment_targets (deployment_id, environment_id, hash)
VALUES (@deployment_id, @environment_id, @hash)
;

-- name: DeploymentTargetsUpdate :exec
UPDATE deployment_targets
SET status = @status, last_modified = now()
WHERE deployment_id = @deployment_id AND environment_id = @environment_id;
;
