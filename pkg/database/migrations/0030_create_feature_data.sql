-- +goose Up
CREATE TABLE feature_data (
    -- Chart.yaml
    "name" TEXT NOT NULL,
    "version" TEXT NOT NULL,
    "chart" TEXT NOT NULL CHECK (chart ~ '^oci://.+$'),
    "description" TEXT NOT NULL,
    "source" TEXT NOT NULL,

    -- Feature.yaml
    "kinds" environment_kind[] NOT NULL,
    "dependencies" JSONB NOT NULL,
    "values" JSONB NOT NULL,

    -- values.yaml
    "default_values" JSONB NOT NULL,

    PRIMARY KEY ("name", "version")
);
