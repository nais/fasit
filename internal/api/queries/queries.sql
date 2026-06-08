-- name: GetFeatureAssignment :one
SELECT
	sqlc.embed(d),
	sqlc.embed(fd)
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.id = @id::UUID;

-- name: ListReconcileStatuses :many
-- Derives a single display status per environment in the rollout vocabulary
-- (pending/deployed/failed/DISABLED) by preferring the deploy_log rollout state
-- and falling back to the latest decision action when the feature was never
-- deployed (e.g. pre-flight failures). disabled_features membership wins.
WITH dep AS (
	SELECT
		environment_id,
		CASE status
		WHEN 'created' THEN
			'pending'
		WHEN 'invalidated' THEN
			'pending'
		ELSE
			status
		END AS status,
		created
	FROM
		deploy_status
	WHERE
		feature_assignment_id = @feature_assignment_id::UUID
),
dec AS (
	SELECT
		environment_id,
		action,
		message,
		created
	FROM
		decision_status
	WHERE
		feature_assignment_id = @feature_assignment_id::UUID
),
disabled AS (
	SELECT
		e.id AS environment_id,
		df.disabled_at
	FROM
		environments e
		JOIN disabled_features df ON df.environment_id = e.id
		JOIN feature_assignments d ON df.feature = d.feature_name
	WHERE
		e.labels @> d.target
		AND d.id = @feature_assignment_id::UUID
),
envs AS (
	SELECT
		environment_id
	FROM
		dep
	UNION
	SELECT
		environment_id
	FROM
		dec
	UNION
	SELECT
		environment_id
	FROM
		disabled
)
SELECT
	@feature_assignment_id::UUID AS feature_assignment_id,
	ev.environment_id,
(
		CASE WHEN dis.environment_id IS NOT NULL THEN
			'DISABLED'
		WHEN dep.status IS NOT NULL THEN
			dep.status
		WHEN dec.action = 'disabled' THEN
			'DISABLED'
		WHEN dec.action IN ('missing-deps', 'missing-config', 'render-error') THEN
			'failed'
		WHEN dec.action IN ('unhealthy', 'in-progress', 'deploy') THEN
			'pending'
		WHEN dec.action = 'unchanged' THEN
			'deployed'
		ELSE
			'unknown'
		END)::TEXT AS status,
	COALESCE(dec.message, '')::TEXT AS message,
	GREATEST(dep.created, dec.created, dis.disabled_at)::TIMESTAMPTZ AS last_modified,
	GREATEST(dep.created, dec.created, dis.disabled_at)::TIMESTAMPTZ AS created
FROM
	envs ev
	LEFT JOIN dep ON dep.environment_id = ev.environment_id
	LEFT JOIN dec ON dec.environment_id = ev.environment_id
	LEFT JOIN disabled dis ON dis.environment_id = ev.environment_id
ORDER BY
	last_modified DESC,
	ev.environment_id ASC;

