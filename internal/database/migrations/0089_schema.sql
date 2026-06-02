-- +goose Up
-- +goose StatementBegin
CREATE TYPE environment_kind AS ENUM(
	'tenant',
	'management',
	'onprem',
	'legacy'
);

CREATE TABLE IF NOT EXISTS audits(
	id UUID DEFAULT gen_random_uuid(),
	actor TEXT NOT NULL,
	description TEXT NOT NULL,
	object_type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
	metadata JSONB,
	action TEXT DEFAULT '' NOT NULL,
	environment_id UUID,
	feature TEXT DEFAULT '' NOT NULL,
	CONSTRAINT audits_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS audits_environment_id_created_at_idx ON audits(environment_id, created_at DESC)
WHERE (environment_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS audits_feature_created_at_idx ON audits(feature, created_at DESC)
WHERE (feature <> ''::TEXT);

CREATE INDEX IF NOT EXISTS audits_feature_environment_id_idx ON audits(feature, environment_id, created_at DESC)
WHERE (feature <> ''::TEXT) AND (environment_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS audits_object_id_idx ON audits USING spgist(object_id);

CREATE TABLE IF NOT EXISTS configurations_global(
	id UUID DEFAULT gen_random_uuid(),
	feature TEXT NOT NULL,
	key text NOT NULL,
	value JSONB NOT NULL,
	description TEXT,
	secret BOOLEAN DEFAULT FALSE NOT NULL,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	CONSTRAINT configurations_global_pkey PRIMARY KEY (id),
	CONSTRAINT configurations_global_unique_idx UNIQUE (feature, key)
);

CREATE TABLE IF NOT EXISTS feature_data(
	name TEXT,
	version TEXT,
	chart TEXT NOT NULL,
	description TEXT NOT NULL,
	source TEXT NOT NULL,
	kinds environment_kind[] NOT NULL,
	dependencies JSONB NOT NULL,
	values jsonb NOT NULL,
	default_values JSONB NOT NULL,
	timeout BIGINT DEFAULT 0 NOT NULL,
	tpl_details JSONB DEFAULT '{}',
	CONSTRAINT feature_data_pkey PRIMARY KEY (NAME, version),
	CONSTRAINT feature_data_chart_check CHECK (chart ~ '^oci://.+$'::TEXT)
);

CREATE TABLE IF NOT EXISTS feature_assignments(
	id UUID DEFAULT gen_random_uuid(),
	feature_name TEXT NOT NULL,
	version TEXT NOT NULL,
	target JSONB DEFAULT '{}' NOT NULL,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	gh_ref JSONB,
	description TEXT,
	active BOOLEAN DEFAULT TRUE NOT NULL,
	CONSTRAINT feature_assignments_pkey PRIMARY KEY (id),
	CONSTRAINT deployments_feature_name_version_fkey FOREIGN KEY (feature_name, version) REFERENCES feature_data(NAME, version)
);

CREATE UNIQUE INDEX IF NOT EXISTS feature_assignments_one_active_per_feature_target ON feature_assignments(feature_name, target)
WHERE (active = TRUE);

CREATE TABLE IF NOT EXISTS tenants(
	id UUID DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	description TEXT,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	last_modified TIMESTAMPTZ DEFAULT now() NOT NULL,
	ci BOOLEAN DEFAULT FALSE NOT NULL,
	CONSTRAINT tenants_pkey PRIMARY KEY (id),
	CONSTRAINT tenants_name_unique UNIQUE (NAME)
);

CREATE UNIQUE INDEX IF NOT EXISTS tenants_ci_idx ON tenants(ci)
WHERE (ci = TRUE);

CREATE TABLE IF NOT EXISTS environments(
	id UUID DEFAULT gen_random_uuid(),
	tenant_id UUID NOT NULL,
	name TEXT NOT NULL,
	kind environment_kind DEFAULT 'tenant'::environment_kind NOT NULL,
	description TEXT,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	last_modified TIMESTAMPTZ DEFAULT now() NOT NULL,
	reconcile BOOLEAN DEFAULT TRUE NOT NULL,
	labels JSONB DEFAULT '{}' NOT NULL,
	CONSTRAINT environments_pkey PRIMARY KEY (id),
	CONSTRAINT environments_name_unique UNIQUE (tenant_id, NAME),
	CONSTRAINT fk_environments_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS configurations_environment(
	id UUID DEFAULT gen_random_uuid() NOT NULL,
	feature TEXT NOT NULL,
	key text NOT NULL,
	value JSONB NOT NULL,
	description TEXT,
	secret BOOLEAN DEFAULT FALSE NOT NULL,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	environment_id UUID NOT NULL,
	CONSTRAINT configurations_environment_unique_idx UNIQUE (environment_id, feature, key),
	CONSTRAINT fk_configurations_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS deploy_instructions(
	id UUID DEFAULT gen_random_uuid(),
	environment_id UUID NOT NULL,
	feature_name TEXT NOT NULL,
	feature_version TEXT NOT NULL,
	status TEXT DEFAULT 'created' NOT NULL,
	hash text NOT NULL,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	last_modified TIMESTAMPTZ DEFAULT now() NOT NULL,
	values jsonb DEFAULT '{}' NOT NULL,
	feature_assignment_id UUID,
	CONSTRAINT deploy_instructions_pkey PRIMARY KEY (id),
	CONSTRAINT deploy_instructions_deployment_id_fkey FOREIGN KEY (feature_assignment_id) REFERENCES feature_assignments(id) ON DELETE CASCADE,
	CONSTRAINT fk_deploy_instructions_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS deploy_instructions_feature_assignment_id_idx ON deploy_instructions(feature_assignment_id);

CREATE INDEX IF NOT EXISTS deploy_instructions_idx ON deploy_instructions(feature_name, feature_version, environment_id);

CREATE TABLE IF NOT EXISTS disabled_features(
	environment_id UUID,
	feature TEXT,
	disabled_at TIMESTAMPTZ DEFAULT now() NOT NULL,
	CONSTRAINT disabled_features_pkey PRIMARY KEY (environment_id, feature),
	CONSTRAINT disabled_features_environment_id_fkey FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS environment_values(
	environment_id UUID,
	key text,
	value JSONB NOT NULL,
	secret BOOLEAN DEFAULT FALSE NOT NULL,
	CONSTRAINT environment_values_pkey PRIMARY KEY (environment_id, key),
	CONSTRAINT environment_values_environment_id_fkey FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS feature_reconcile_statuses(
	feature_assignment_id UUID,
	environment_id UUID,
	status TEXT DEFAULT 'pending' NOT NULL,
	message TEXT NOT NULL,
	last_modified TIMESTAMPTZ DEFAULT now() NOT NULL,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	CONSTRAINT feature_reconcile_statuses_pkey PRIMARY KEY (feature_assignment_id, environment_id),
	CONSTRAINT deployment_statuses_deployment_id_fkey FOREIGN KEY (feature_assignment_id) REFERENCES feature_assignments(id) ON DELETE CASCADE,
	CONSTRAINT deployment_statuses_environment_id_fkey FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS health_statuses(
	environment_id UUID,
	reported_at TIMESTAMPTZ DEFAULT now() NOT NULL,
	CONSTRAINT health_statuses_pkey PRIMARY KEY (environment_id),
	CONSTRAINT fk_health_statuses_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS logs(
	id BIGSERIAL,
	deploy_instruction UUID NOT NULL,
	time TIMESTAMPTZ NOT NULL,
	message TEXT NOT NULL,
	kind TEXT DEFAULT '' NOT NULL,
	CONSTRAINT logs_pkey PRIMARY KEY (id),
	CONSTRAINT logs_deploy_instruction_fkey FOREIGN KEY (deploy_instruction) REFERENCES deploy_instructions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS release_statuses(
	environment_id UUID,
	feature TEXT,
	version TEXT NOT NULL,
	status TEXT NOT NULL,
	revision INTEGER NOT NULL,
	last_deployed TIMESTAMPTZ NOT NULL,
	created TIMESTAMPTZ DEFAULT now() NOT NULL,
	last_modified TIMESTAMPTZ DEFAULT now() NOT NULL,
	CONSTRAINT release_statuses_pkey PRIMARY KEY (environment_id, feature),
	CONSTRAINT fk_release_statuses_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION prevent_modify()
	RETURNS TRIGGER
	LANGUAGE plpgsql
	VOLATILE
	AS $$
BEGIN
	RAISE EXCEPTION 'Cannot modify rows in %', TG_TABLE_NAME;
END;
$$;

CREATE OR REPLACE FUNCTION update_modified_timestamp()
	RETURNS TRIGGER
	LANGUAGE plpgsql
	VOLATILE
	AS $$
BEGIN
	NEW.last_modified = NOW();
	RETURN NEW;
END;
$$;

CREATE OR REPLACE TRIGGER audits_no_modify
	BEFORE UPDATE OR DELETE ON audits
	FOR EACH ROW
	EXECUTE FUNCTION prevent_modify();

CREATE OR REPLACE TRIGGER deploy_instructions_set_modified
	BEFORE UPDATE ON deploy_instructions
	FOR EACH ROW
	EXECUTE FUNCTION update_modified_timestamp();

CREATE OR REPLACE TRIGGER environments_set_modified
	BEFORE UPDATE ON environments
	FOR EACH ROW
	EXECUTE FUNCTION update_modified_timestamp();

CREATE OR REPLACE TRIGGER feature_reconcile_statuses_set_modified
	BEFORE UPDATE ON feature_reconcile_statuses
	FOR EACH ROW
	EXECUTE FUNCTION update_modified_timestamp();

CREATE OR REPLACE TRIGGER release_statuses_set_modified
	BEFORE UPDATE ON release_statuses
	FOR EACH ROW
	EXECUTE FUNCTION update_modified_timestamp();

CREATE OR REPLACE TRIGGER tenant_set_modified
	BEFORE UPDATE ON tenants
	FOR EACH ROW
	EXECUTE FUNCTION update_modified_timestamp();

-- +goose StatementEnd
