-- +goose Up
CREATE TABLE disabled_features(
	"environment_id" UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
	"feature" TEXT NOT NULL,
	"disabled_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (environment_id, feature)
);
