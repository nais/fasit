-- name: FeatureVersions :many
SELECT
	fd.name,
	fd.version,
	fd.description,
	fd.source,
	MAX(COALESCE(fa.created, '0001-01-01 00:00:00+00'))::TIMESTAMPTZ AS last_updated
FROM
	feature_data fd
	LEFT JOIN feature_assignments fa ON fa.feature_name = fd.name
		AND fa.version = fd.version
WHERE
	fd.name = @feature_name
GROUP BY
	fd.name,
	fd.version,
	fd.description,
	fd.source
ORDER BY
	last_updated DESC,
	fd.version DESC;

-- name: ListActiveFeatures :many
SELECT DISTINCT ON (d.feature_name)
	fd.name,
	fd.description,
	fd.source
FROM
	feature_assignments d
	JOIN feature_data fd ON d.feature_name = fd.name
		AND d.version = fd.version
WHERE
	d.active = TRUE
ORDER BY
	d.feature_name,
	d.created DESC;

