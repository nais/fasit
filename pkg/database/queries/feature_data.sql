-- name: FeatureDataCreate :exec
INSERT INTO feature_data (
    name,
    version,
    chart,
    description,
    source,
    kinds,
    dependencies,
    values,
    default_values,
    timeout,
    tpl_details,
    moved
) VALUES (
    @feature_name,
    @version,
    @chart,
    @description,
    @source,
    (@kinds::text[])::environment_kind[],
    @dependencies,
    @values,
    @default_values,
    @timeout,
    @tpl_details,
    @moved
);
