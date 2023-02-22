-- name: FeaturesForKind :many
SELECT * FROM features WHERE @environment_kind::text = ANY(environment_kinds) ORDER BY name; 