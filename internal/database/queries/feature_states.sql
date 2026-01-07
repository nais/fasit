-- name: FeatureStatesGet :many
WITH env AS (
	SELECT
		ci,
		kind
	FROM
		environments
	WHERE
		id = @environment_id
),
combined AS (
	SELECT
		NULL AS id,
		name,
		version,
		last_modified
	FROM
		features
	UNION
	SELECT
		id,
		feature_name AS name,
		version,
		NULL AS last_modified
	FROM
		rollouts
	WHERE
		status = 'pending'
),
filtered AS (
	SELECT DISTINCT ON (name)
		name,
		version
	FROM
		combined
		JOIN env ON 1 = 1
	ORDER BY
		name,
		-- If environment is CI, use definition from rollouts if it exists, otherwise use definition from features
		CASE WHEN env.ci THEN
			id
		END,
		CASE WHEN NOT ci THEN
			last_modified
		END
)
SELECT
	@environment_id::UUID AS environment_id,
	f.name,
	COALESCE(fs.enabled, FALSE) AS enabled,
	fs.created,
	fs.last_modified,
	fs.enabled_at
FROM
	filtered f
	JOIN feature_data fd ON fd.name = f.name
		AND fd.version = f.version
	LEFT JOIN feature_states fs ON fs.feature = f.name
		AND fs.environment_id = @environment_id
WHERE (
	SELECT
		kind
	FROM
		env) = ANY (fd.kinds)
ORDER BY
	f.name ASC;

-- name: FeatureStatesGetOld :many
SELECT
	*
FROM
	feature_states
	LEFT JOIN features ON features.name = feature_states.feature
WHERE
	environment_id = @environment_id
	AND features.name IS NULL
ORDER BY
	feature ASC;

-- name: RolloutStatesGet :many
SELECT
	@environment_id::UUID AS environment_id,
	r.feature_name,
	COALESCE(fs.enabled, FALSE) AS enabled,
	fs.created,
	fs.last_modified,
	fs.enabled_at
FROM
	rollouts r
	JOIN feature_data fd ON fd.name = r.feature_name
		AND fd.version = r.version
	LEFT JOIN feature_states fs ON fs.feature = r.feature_name
		AND fs.environment_id = @environment_id
WHERE (
	SELECT
		kind
	FROM
		environments
	WHERE
		id = @environment_id) = ANY (fd.kinds)
ORDER BY
	r.feature_name ASC;

-- name: FeatureStateGet :one
SELECT
	*
FROM
	feature_states
WHERE
	feature = @feature
	AND environment_id = @environment_id;

-- name: FeatureStateCreateOrUpdate :one
INSERT INTO feature_states(
	environment_id,
	feature,
	enabled,
	enabled_at)
VALUES (
	@environment_id,
	@feature,
	@enabled,
	@enabledAt)
ON CONFLICT (
	environment_id,
	feature)
	DO UPDATE SET
		enabled = EXCLUDED.enabled,
		enabled_at = EXCLUDED.enabled_at
	RETURNING
		*;

