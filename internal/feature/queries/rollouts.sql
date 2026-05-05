-- name: RolloutsForKind :many
WITH success AS (
	SELECT DISTINCT ON (rollouts.feature_name)
		id,
		feature_name
	FROM
		rollouts
		JOIN feature_data fd ON rollouts.feature_name = fd.name
			AND rollouts.version = fd.version
	WHERE
		@environment_kind::environment_kind = ANY (kinds)
		AND (rollouts.status IN ('pending', 'in_progress', 'deployed'))
	ORDER BY
		rollouts.feature_name,
		rollouts.created DESC
),
failed AS (
	SELECT DISTINCT ON (rollouts.feature_name)
		rollouts.id
	FROM
		rollouts
		LEFT OUTER JOIN success ON success.feature_name = rollouts.feature_name
	JOIN feature_data fd ON rollouts.feature_name = fd.name
		AND rollouts.version = fd.version
	WHERE
		success.id IS NULL
		AND @environment_kind::environment_kind = ANY (kinds)
		AND (rollouts.status IN ('failed'))
	ORDER BY
		rollouts.feature_name,
		rollouts.created DESC
),
all_rollouts AS (
	SELECT
		id
	FROM
		success
	UNION
	SELECT
		id
	FROM
		failed
)
SELECT
	rollouts.id,
	sqlc.embed(fd),
	rollouts.created,
	EXISTS (
		SELECT
			1
		FROM
			deployments d
		WHERE
			d.feature_name = fd.name) AS hasDeployments
	FROM
		rollouts
		JOIN all_rollouts ar ON ar.id = rollouts.id
		JOIN feature_data fd ON rollouts.feature_name = fd.name
			AND rollouts.version = fd.version
		ORDER BY
			rollouts.feature_name ASC;

-- name: RolloutByName :one
SELECT
	rollouts.id,
	sqlc.embed(fd),
	rollouts.created
FROM
	rollouts
	JOIN feature_data fd ON rollouts.feature_name = fd.name
		AND rollouts.version = fd.version
WHERE
	fd.name = @name
	AND rollouts.status = 'pending';

