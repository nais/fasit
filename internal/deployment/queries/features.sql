-- name: ListFeatureStatesInEnvironment :many
SELECT DISTINCT
	d.feature_name,
	EXISTS (
		SELECT
			1
		FROM
			disabled_features df
		WHERE
			df.environment_id = e.id
			AND df.feature = d.feature_name) AS disabled
FROM
	deployments d
	JOIN environments e ON e.id = @environment_id
		AND e.labels @> d.target -- @> operator checks if the JSONB on the left contains the JSONB on the right
	ORDER BY
		d.feature_name;

-- name: ListFeatures :many
SELECT DISTINCT
	feature_name
FROM
	deployments
ORDER BY
	feature_name;

