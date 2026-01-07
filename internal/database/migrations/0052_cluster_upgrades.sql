-- +goose Up
CREATE TYPE cluster_upgrades_status AS ENUM(
	'CREATED',
	'MASTER_UPGRADE',
	'NODE_UPGRADE',
	'FAILED',
	'DONE'
);

CREATE TABLE cluster_upgrades(
	"id" UUID DEFAULT uuid_generate_v4(),
	"tenant_id" UUID NOT NULL,
	"environment_id" UUID NOT NULL,
	"version" TEXT NOT NULL,
	"status" cluster_upgrades_status NOT NULL DEFAULT 'CREATED',
	"start_time" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	CONSTRAINT fk_cluster_upgrades_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
	CONSTRAINT fk_cluster_upgrades_env FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TRIGGER cluster_upgrades_set_modified
	BEFORE UPDATE ON cluster_upgrades
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

CREATE TRIGGER cluster_upgrades_notify
	AFTER INSERT OR UPDATE ON cluster_upgrades
	FOR EACH ROW
	EXECUTE PROCEDURE fasit_notify("id");

