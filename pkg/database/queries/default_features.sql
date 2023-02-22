-- name: DefaultFeaturesForKind :many
SELECT * FROM default_features WHERE kind = @environment_kind ORDER BY feature;