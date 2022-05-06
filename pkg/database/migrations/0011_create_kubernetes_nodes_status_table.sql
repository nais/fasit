-- +goose Up
CREATE TABLE kubernetes_node_statuses
(
    "environment_id" uuid,
    name text NOT NULL,
    kernel_version text NOT NULL,
    os_image text NOT NULL,
    container_runtime_version text NOT NULL,
    kubelet_version text NOT NULL,
    kube_proxy_version text NOT NULL,
    operating_system text NOT NULL,
    architecture text NOT NULL,
    conditions jsonb NOT NULL,
    allocatable jsonb NOT NULL,
    capacity jsonb NOT NULL,
    "created"        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "last_modified"  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (environment_id, name),
    CONSTRAINT fk_release_statuses_environments
        FOREIGN KEY (environment_id)
        REFERENCES environments (id) ON DELETE CASCADE
);

CREATE TRIGGER kubernetes_node_statuses_set_modified
    BEFORE UPDATE
    ON kubernetes_node_statuses
    FOR EACH ROW
    EXECUTE PROCEDURE update_modified_timestamp();
