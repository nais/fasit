-- name: FeatureStatesGet :many
SELECT @environment_id::uuid AS environment_id, f.name, coalesce(fs.enabled, false) AS enabled, fs.created, fs.last_modified, fs.enabled_at
FROM features f
JOIN feature_data fd ON fd.name = f.name AND fd.version = f.version
LEFT JOIN feature_states fs
ON fs.feature = f.name AND fs.environment_id = @environment_id
WHERE (SELECT kind FROM environments WHERE id = @environment_id) = any(fd.kinds)
;

-- name: FeatureStateGet :one
SELECT *
FROM feature_states
WHERE feature = @feature AND environment_id = @environment_id;

-- name: FeatureStateCreateOrUpdate :one
INSERT INTO feature_states
(environment_id, feature, enabled, enabled_at)
VALUES
	(@environment_id, @feature, @enabled, @enabledAt)
ON CONFLICT (environment_id, feature) DO UPDATE
	SET
		enabled = EXCLUDED.enabled,
		enabled_at = EXCLUDED.enabled_at
RETURNING *;
