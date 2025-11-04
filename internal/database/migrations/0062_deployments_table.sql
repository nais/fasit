-- +goose Up
CREATE TABLE "deployments" (
	"id" uuid PRIMARY KEY DEFAULT uuid_generate_v4 () NOT NULL,
	"feature_name" TEXT NOT NULL,
	"version" TEXT NOT NULL,
	"target" jsonb NOT NULL,
	"created" TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	"gh_ref" jsonb,
	"deploy_instructions" uuid[] DEFAULT '{}' NOT NULL,
	FOREIGN KEY (feature_name, version) REFERENCES feature_data (name, version)
)
;

CREATE TABLE "deployment_targets" (
	"deployment_id" uuid NOT NULL REFERENCES deployments (id) ON DELETE CASCADE,
	"environment_id" uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
	"status" TEXT NOT NULL DEFAULT 'pending',
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"created" TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	"hash" TEXT NOT NULL,
	PRIMARY KEY ("deployment_id", "environment_id")
)
;
