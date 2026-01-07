-- name: AutoInstallsForKind :many
SELECT
	*
FROM
	auto_installs
WHERE
	kind = @environment_kind
ORDER BY
	feature;

-- name: AutoInstallNamesForKind :many
SELECT
	feature
FROM
	auto_installs
WHERE
	kind = @environment_kind
ORDER BY
	feature;

