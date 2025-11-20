-- +goose Up
CREATE TABLE "deployments" (
	"id" uuid PRIMARY KEY DEFAULT uuid_generate_v4 () NOT NULL,
	"feature_name" TEXT NOT NULL,
	"version" TEXT NOT NULL,
	"target" jsonb NOT NULL DEFAULT '{}',
	"created" TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	"gh_ref" jsonb,
	FOREIGN KEY (feature_name, version) REFERENCES feature_data (name, version)
)
;

CREATE TABLE "environment_features" (
	"environment_id" uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
	"feature_name" TEXT NOT NULL,
	"feature_version" TEXT NOT NULL,
	"deployment_id" uuid NOT NULL REFERENCES deployments (id) ON DELETE CASCADE,
	PRIMARY KEY (environment_id, feature_name),
	FOREIGN KEY (feature_name, feature_version) REFERENCES feature_data (name, version) ON DELETE CASCADE
)
;

CREATE TABLE "deployment_statuses" (
	"deployment_id" uuid NOT NULL REFERENCES deployments (id) ON DELETE CASCADE,
	"environment_id" uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
	"status" TEXT NOT NULL DEFAULT 'pending',
	"message" TEXT NOT NULL,
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"created" TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	PRIMARY KEY ("deployment_id", "environment_id")
)
;

CREATE TRIGGER deployment_statuses_set_modified BEFORE
UPDATE ON deployment_statuses FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp ()
;

ALTER TABLE environments
ADD COLUMN IF NOT EXISTS labels JSONB NOT NULL DEFAULT '{}'::JSONB
;

UPDATE environments e
SET
	labels = el.labels
FROM
	(
		SELECT
			environment_id,
			JSONB_OBJECT_AGG(key, value) AS labels
		FROM
			environment_labels
		GROUP BY
			environment_id
	) el
WHERE
	e.id = el.environment_id
;

DROP TABLE IF EXISTS environment_labels
;

ALTER TABLE deploy_instructions
ADD COLUMN deployment_id uuid REFERENCES deployments (id) ON DELETE CASCADE
;

CREATE INDEX ON "deploy_instructions" ("deployment_id")
;
