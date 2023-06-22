-- name: FeatureDataCreate :exec
INSERT INTO feature_data (
    name,
    version,
    chart,
    description,
    source,
    kinds,
    timeout,
    dependencies,
    values,
    default_values
) VALUES (
    @feature_name,
    @version,
    @chart,
    @description,
    @source,
    (@kinds::text[])::environment_kind[],
    @timeout,
    @dependencies,
    @values,
    @default_values
);
