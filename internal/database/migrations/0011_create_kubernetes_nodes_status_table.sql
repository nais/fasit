-- +goose Up
CREATE TABLE kubernetes_node_statuses(
	"environment_id" UUID,
	name TEXT NOT NULL,
	kernel_version TEXT NOT NULL,
	os_image TEXT NOT NULL,
	container_runtime_version TEXT NOT NULL,
	kubelet_version TEXT NOT NULL,
	kube_proxy_version TEXT NOT NULL,
	operating_system TEXT NOT NULL,
	architecture TEXT NOT NULL,
	conditions JSONB NOT NULL,
	allocatable JSONB NOT NULL,
	capacity JSONB NOT NULL,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (environment_id, NAME),
	CONSTRAINT fk_release_statuses_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TRIGGER kubernetes_node_statuses_set_modified
	BEFORE UPDATE ON kubernetes_node_statuses
	FOR EACH ROW
	EXECUTE PROCEDURE update_modified_timestamp();

