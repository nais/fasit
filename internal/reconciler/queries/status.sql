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

-- name: ListAllDecisionStatuses :many
-- Latest reconciler decision per environment for every feature assignment.
-- Grouped by feature_assignment_id in Go (AllReconcileStatuses) to avoid a
-- per-assignment query fan-out.
SELECT
	feature_assignment_id,
	environment_id,
	action,
	message,
	created
FROM
	decision_status
ORDER BY
	feature_assignment_id ASC,
	environment_id ASC;

-- name: ListAllDeployStatuses :many
-- Latest deploy rollout state per environment for every feature assignment.
SELECT
	feature_assignment_id,
	environment_id,
	status,
	created
FROM
	deploy_status
ORDER BY
	feature_assignment_id ASC,
	environment_id ASC;

-- name: ListRecentDeploys :many
-- Recent deploy log rows, newest first. Per-instruction deduplication and
-- aggregation by feature version happen in Go (ListRecentDeploys).
SELECT
	diid,
	feature_name,
	feature_version,
	status,
	feature_assignment_id,
	created
FROM
	deploy_log
ORDER BY
	created DESC,
	id DESC
LIMIT @lim::INT;

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

