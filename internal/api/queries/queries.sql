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

-- name: ListReconcileSignals :many
-- Returns the raw status signals per environment for a feature assignment: the
-- deploy rollout state, the latest reconciler decision, and disabled-feature
-- membership. The effective display status is selected in Go
-- (featureassignment.DeriveReconcileState).
WITH dep AS (
	SELECT
		environment_id,
		status,
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
	ev.environment_id,
	COALESCE(dep.status, '')::TEXT AS deploy_status,
	COALESCE(dec.action, '')::TEXT AS decision_action,
	COALESCE(dec.message, '')::TEXT AS decision_message,
(dis.environment_id IS NOT NULL)::BOOL AS disabled,
	GREATEST(dep.created, dec.created, dis.disabled_at)::TIMESTAMPTZ AS last_modified
FROM
	envs ev
	LEFT JOIN dep ON dep.environment_id = ev.environment_id
	LEFT JOIN dec ON dec.environment_id = ev.environment_id
	LEFT JOIN disabled dis ON dis.environment_id = ev.environment_id
ORDER BY
	ev.environment_id ASC;

