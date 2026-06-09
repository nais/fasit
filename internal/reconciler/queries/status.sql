-- name: ListDecisionStatuses :many
-- Latest reconciler decision per environment for a feature assignment. The
-- deploy rollout state and disabled-feature membership are joined in Go
-- (ReconcileStatuses).
SELECT
	environment_id,
	action,
	message,
	created
FROM
	decision_status
WHERE
	feature_assignment_id = @feature_assignment_id::UUID
ORDER BY
	environment_id ASC;

-- name: ListDeployStatuses :many
-- Latest deploy rollout state per environment for a feature assignment.
SELECT
	environment_id,
	status,
	created
FROM
	deploy_status
WHERE
	feature_assignment_id = @feature_assignment_id::UUID
ORDER BY
	environment_id ASC;

-- name: ListDisabledEnvironments :many
-- Environments the assignment targets where the feature is disabled.
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
ORDER BY
	environment_id ASC;


-- name: ListDecisionLog :many
-- Decision history for a feature in an environment, newest first. Rows exist
-- only for cycles where the decision changed.
SELECT
	id,
	feature_assignment_id,
	feature_version,
	action,
	message,
	created
FROM
	decision_log
WHERE
	environment_id = @environment_id::UUID
	AND feature_name = @feature_name::TEXT
ORDER BY
	created DESC,
	id DESC
LIMIT 50;
