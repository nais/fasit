-- +goose Up
CREATE TYPE cluster_version_status AS ENUM ('created', 'master_upgrade', 'node_upgrade', 'failed', 'done');

CREATE TABLE cluster_version (
    "id" UUID DEFAULT uuid_generate_v4(),
    "tenant_id" UUID NOT NULL,
    "environment_id" UUID NOT NULL,
    "version" text NOT NULL,
    "status" cluster_version_status NOT NULL DEFAULT 'created',
    "last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
    CONSTRAINT fk_cluster_version_tenant
      FOREIGN KEY (tenant_id)
          REFERENCES tenants (id) ON DELETE CASCADE,
    CONSTRAINT fk_cluster_version_env
      FOREIGN KEY (environment_id)
          REFERENCES environments (id) ON DELETE CASCADE
);

CREATE TRIGGER cluster_version_set_modified
    BEFORE UPDATE
    ON cluster_version
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();
