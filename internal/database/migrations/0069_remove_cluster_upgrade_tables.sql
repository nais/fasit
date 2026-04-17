-- +goose Up
DROP TABLE IF EXISTS cluster_operations;

DROP TABLE IF EXISTS cluster_upgrades;

DROP TYPE IF EXISTS cluster_upgrades_status;

-- +goose Down
CREATE TYPE cluster_upgrades_status AS ENUM(
	'CREATED',
	'WAITING',
	'CONTROL_PLANE_UPGRADE',
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
	"slack_message_timestamp" TEXT,
	"slack_channel_id" TEXT,
	"is_automatic" BOOLEAN DEFAULT NULL,
	"upgrade_start_time" TIMESTAMPTZ,
	PRIMARY KEY (id),
	CONSTRAINT fk_cluster_upgrades_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
	CONSTRAINT fk_cluster_upgrades_env FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX cluster_upgrades_one_in_progress ON cluster_upgrades(environment_id)
WHERE
	status IN ('CREATED', 'WAITING', 'CONTROL_PLANE_UPGRADE', 'NODE_UPGRADE');

CREATE TRIGGER cluster_upgrades_set_modified
	BEFORE UPDATE ON cluster_upgrades
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

CREATE TRIGGER cluster_upgrades_notify
	AFTER INSERT OR UPDATE ON cluster_upgrades
	FOR EACH ROW
	EXECUTE PROCEDURE fasit_notify("id");

CREATE TABLE cluster_operations(
	"id" UUID PRIMARY KEY,
	"operation_name" TEXT NOT NULL,
	"tenant_id" UUID NOT NULL,
	"environment_id" UUID NOT NULL,
	"upgrade_id" UUID NOT NULL,
	"status" TEXT NOT NULL,
	"type" TEXT NOT NULL,
	"detail" TEXT NOT NULL,
	"target" TEXT NOT NULL,
	"nodes_total" INT NOT NULL,
	"nodes_failed" INT NOT NULL,
	"nodes_completed" INT NOT NULL,
	"nodes_done" INT NOT NULL,
	"node_pdb_delay_seconds" INT NOT NULL,
	"start_time" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT fk_cluster_operations_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
	CONSTRAINT fk_cluster_operations_env FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
	CONSTRAINT fk_cluster_operations_version FOREIGN KEY (upgrade_id) REFERENCES cluster_upgrades(id) ON DELETE CASCADE
);

CREATE TRIGGER cluster_operations_set_modified
	BEFORE UPDATE ON cluster_operations
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

