-- name: DefaultFeaturesForKind :many
SELECT feature FROM default_features WHERE kind = @environment_kind ORDER BY feature;