-- +goose Up
CREATE TABLE "deployment" (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,

    "feature_name" text NOT NULL,
    "version" text NOT NULL,
    "target" jsonb NOT NULL,

    "created" timestamptz DEFAULT now() NOT NULL,
    "gh_ref" jsonb,
    "deploy_instructions" uuid[] DEFAULT '{}' NOT NULL,

    FOREIGN KEY (feature_name, version) REFERENCES feature_data(name, version) NOT DEFERRABLE
)
;

CREATE TABLE "deployment_targets" (
   "deployment_id" uuid NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
   "environment_id" uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
   "status" text NOT NULL default 'pending',
   "last_modified" timestamptz NOT NULL DEFAULT now(),
   "created" timestamptz DEFAULT now() NOT NULL,
   "hash" text NOT NULL,

   PRIMARY KEY ("deployment_id", "environment_id")
)
;
