-- +goose Up

CREATE TABLE cluster_upgrade (
    "operation_id" TEXT PRIMARY KEY,
    "tenant_id" UUID NOT NULL,
    "environment_id" UUID NOT NULL,
    "status" TEXT NOT NULL,
    "type" TEXT NOT NULL,
    "master_version" TEXT,
    "nodes_total" INT NOT NULL,
    "nodes_failed" INT NOT NULL,
    "nodes_completed" INT NOT NULL,
    "nodes_done" INT NOT NULL,
    "node_pdb_delay_seconds" INT NOT NULL,
    "start_time" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cluster_upgrade_unique_idx UNIQUE (operation_id,tenant_id,environment_id),
    CONSTRAINT fk_cluster_upgrade_tenant
      FOREIGN KEY (tenant_id)
          REFERENCES tenants (id) ON DELETE CASCADE,
    CONSTRAINT fk_cluster_upgrade_env
      FOREIGN KEY (environment_id)
          REFERENCES environments (id) ON DELETE CASCADE
);

CREATE TRIGGER cluster_upgrade_set_modified
    BEFORE UPDATE
    ON cluster_upgrade
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();
