-- +goose Up
CREATE TABLE cluster_operations (
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
	CONSTRAINT fk_cluster_operations_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE CASCADE,
	CONSTRAINT fk_cluster_operations_env FOREIGN KEY (environment_id) REFERENCES environments (id) ON DELETE CASCADE,
	CONSTRAINT fk_cluster_operations_version FOREIGN KEY (upgrade_id) REFERENCES cluster_upgrades (id) ON DELETE CASCADE
)
;

CREATE TRIGGER cluster_operations_set_modified BEFORE
UPDATE ON cluster_operations FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp ()
;
