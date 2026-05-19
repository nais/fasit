-- name: Warnings :many
WITH latest_di AS (
	SELECT DISTINCT ON (feature_name,
		environment_id)
		'feature_status' AS "type",
		di.environment_id,
		environment.tenant_id,
		feature_name,
		CASE WHEN fd.name IS NULL THEN
			''
		ELSE
			fd.name
		END AS feature_data_name,
		status
	FROM
		deploy_instructions di
		JOIN environments environment ON environment.id = di.environment_id
		LEFT JOIN disabled_features df ON df.environment_id = di.environment_id
			AND df.feature = di.feature_name
		LEFT JOIN features fd ON fd.name = di.feature_name
			AND fd.version = di.feature_version
	WHERE (environment.id = @environment_id
		OR environment.tenant_id = @tenant_id)
		AND df.feature IS NULL
ORDER BY
	feature_name,
	di.environment_id,
	di.last_modified DESC
)
SELECT
	type,
	environment_id,
	tenant_id,
	feature_name,
	feature_data_name
FROM
	latest_di
WHERE
	status = 'failed'
UNION
SELECT
	'naisd',
	environment.id,
	environment.tenant_id,
	'',
	'naisd'
FROM
	health_statuses hs
	RIGHT JOIN environments environment ON environment.id = hs.environment_id
WHERE (hs.reported_at IS NULL
	OR hs.reported_at < NOW() - INTERVAL '10 minutes')
AND (environment.id = @environment_id
	OR environment.tenant_id = @tenant_id);

