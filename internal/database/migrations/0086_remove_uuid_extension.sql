-- +goose Up
ALTER TABLE tenants
	ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE environments
	ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE configurations_global
	ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE configurations_environment
	ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE audits
	ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE feature_assignments
	ALTER COLUMN id SET DEFAULT gen_random_uuid();

DROP EXTENSION IF EXISTS "uuid-ossp";

