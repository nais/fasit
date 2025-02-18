-- +goose Up
CREATE TABLE configurations (
	"id" uuid NOT NULL DEFAULT uuid_generate_v4 (),
	"environment_id" uuid,
	"feature" TEXT NOT NULL,
	"key" TEXT NOT NULL,
	"value" JSONB NOT NULL,
	"description" TEXT,
	"secret" BOOLEAN NOT NULL DEFAULT FALSE,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	CONSTRAINT configurations_unique_idx UNIQUE (environment_id, feature, key)
)
;
