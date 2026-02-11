-- name: RolloutDelete :exec
DELETE FROM rollouts
WHERE feature_name = @feature_name;

-- name: RolloutCreate :one
INSERT INTO rollouts(
	feature_name,
	version,
	gh_ref)
VALUES (
	@feature_name,
	@version,
	@gh_ref)
RETURNING
	*;

-- name: RolloutByID :one
SELECT
	*
FROM
	rollouts
WHERE
	id = @id;

-- name: RolloutUpdateStatus :exec
UPDATE
	rollouts
SET
	status = @status
WHERE
	feature_name = @feature_name
	AND completed IS NULL;

-- name: RolloutEventCreate :exec
INSERT INTO rollout_events(
	rollout_id,
	failure,
	message,
	data)
VALUES (
	@rollout_id,
	@failure::BOOLEAN,
	@message,
	@data);

-- name: RolloutStatus :one
SELECT
	status
FROM
	rollouts
WHERE
	feature_name = @feature_name
	AND completed IS NULL;

-- name: RolloutComplete :exec
UPDATE
	rollouts
SET
	completed = NOW()
WHERE
	feature_name = @feature_name
	AND completed IS NULL;

-- name: RolloutsForFeature :many
SELECT
	*
FROM
	rollouts
WHERE
	feature_name = @feature_name
ORDER BY
	created DESC
LIMIT 30;

-- name: RolloutByNameAndVersion :one
SELECT
	*
FROM
	rollouts
WHERE
	feature_name = @feature_name
	AND version = @version;

-- name: RolloutEventForRollout :many
SELECT
	*
FROM
	rollout_events
WHERE
	rollout_id = @rollout_id
ORDER BY
	created ASC;

-- name: RolloutCalculateDone :one
WITH rollout AS (
	SELECT
		*
	FROM
		rollouts
	WHERE
		rollouts.id = @rollout_id
),
dis AS (
	SELECT
		di.*
	FROM
		deploy_instructions di
		INNER JOIN rollout ON di.feature_name = rollout.feature_name
			AND di.feature_version = rollout.version
	WHERE
		di.status IN ('deployed', 'failed')
),
cienvs AS (
	SELECT
		id
	FROM
		environments
	WHERE
		ci = TRUE
),
feature_states AS (
	SELECT
		COUNT(1)
	FROM
		feature_states
	WHERE
		feature =(
			SELECT
				feature_name
			FROM
				rollout)
			AND environment_id IN (
				SELECT
					id
				FROM
					cienvs)
				AND enabled = TRUE
)
	SELECT
		(
			SELECT
				COUNT(1)
			FROM
				dis) =(
		SELECT
			*
		FROM
			feature_states) AS done;

-- name: RolloutMarkFailed :execrows
UPDATE
	rollouts
SET
	status = 'failed',
	completed = NOW()
WHERE
	id = @rollout_id
	AND status NOT IN ('deployed', 'failed');

-- name: Rollouts :many
SELECT
	*
FROM
	rollouts
ORDER BY
	created DESC
LIMIT sqlc.arg('limit');

