-- name: AutoInstallsForKind :many
SELECT feature FROM auto_installs WHERE kind = @environment_kind ORDER BY feature;