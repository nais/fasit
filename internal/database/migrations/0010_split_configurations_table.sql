-- +goose Up
CREATE TABLE configurations_global(
	"id" UUID NOT NULL DEFAULT uuid_generate_v4(),
	"feature" TEXT NOT NULL,
	"key" TEXT NOT NULL,
	"value" JSONB NOT NULL,
	"description" TEXT,
	"secret" BOOLEAN NOT NULL DEFAULT FALSE,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	CONSTRAINT configurations_global_unique_idx UNIQUE (feature, key)
);

CREATE TABLE configurations_environment(
	"environment_id" UUID NOT NULL,
	CONSTRAINT configurations_environment_unique_idx UNIQUE (environment_id, feature, key),
	CONSTRAINT fk_configurations_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
)
INHERITS (
	configurations_global
);

-- Move data from old table to new ones
INSERT INTO configurations_global(
	id,
	feature,
	key,
	value,
	description,
	secret,
	created)
SELECT
	id,
	feature,
	key,
	value,
	description,
	secret,
	created
FROM
	configurations
WHERE
	environment_id IS NULL;

INSERT INTO configurations_environment(
	id,
	feature,
	key,
	value,
	description,
	secret,
	created,
	environment_id)
SELECT
	id,
	feature,
	key,
	value,
	description,
	secret,
	created,
	environment_id
FROM
	configurations
WHERE
	environment_id IS NOT NULL;

DROP TABLE configurations;

