-- name: KubernetesNodeCreateOrUpdate :exec
INSERT INTO kubernetes_node_statuses
	(
        environment_id,
        name,
        kernel_version,
        os_image,
        container_runtime_version,
        kubelet_version,
        kube_proxy_version,
        operating_system,
        architecture,
        conditions,
        allocatable,
        capacity,
        internal_ip
    )
VALUES
	(
        @environment_id,
        @name,
        @kernel_version,
        @os_image,
        @container_runtime_version,
        @kubelet_version,
        @kube_proxy_version,
        @operating_system,
        @architecture,
        @conditions,
        @allocatable,
        @capacity,
        @internal_ip
    )
ON CONFLICT (environment_id, name) DO UPDATE
	SET
    kernel_version = EXCLUDED.kernel_version,
    os_image = EXCLUDED.os_image,
    container_runtime_version = EXCLUDED.container_runtime_version,
    kubelet_version = EXCLUDED.kubelet_version,
    kube_proxy_version = EXCLUDED.kube_proxy_version,
    operating_system = EXCLUDED.operating_system,
    architecture = EXCLUDED.architecture,
    conditions = EXCLUDED.conditions,
    allocatable = EXCLUDED.allocatable,
    capacity = EXCLUDED.capacity,
    internal_ip = EXCLUDED.internal_ip
;

-- name: KubernetesNodeDeleteObsolete :exec
DELETE FROM kubernetes_node_statuses WHERE environment_id = @environment_id AND last_modified < NOW() - INTERVAL '1 minute';

-- name: KubernetesNodeStatuses :many
SELECT * FROM kubernetes_node_statuses
WHERE environment_id = @environment_ID
ORDER BY name ASC
;
