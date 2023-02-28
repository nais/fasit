-- name: FeatureByName :one
SELECT *
FROM features 
WHERE name = @name
ORDER BY name;

-- name: FeaturesForKind :many
SELECT feature_data.*, features.created, features.last_modified 
FROM features 
JOIN feature_data ON features.name = feature_data.name AND features.version = feature_data.version
WHERE @environment_kind::text = ANY(environment_kinds) 
ORDER BY features.name;