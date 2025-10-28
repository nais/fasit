-- +goose Up
CREATE TABLE environment_labels (
    "environment_id" uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
    "key" TEXT NOT NULL,
    "value" TEXT NOT NULL,
    PRIMARY KEY ("environment_id", "key")
)
;
