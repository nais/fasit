package local

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var phases = []corev1.NodePhase{
	corev1.NodeRunning,
	corev1.NodeRunning,
	corev1.NodePending,
	corev1.NodeTerminated,
}

func NewKubernetesClient() kubernetes.Interface {
	objs := []runtime.Object{}
	for i := 0; i < 5; i++ {
		n := &corev1.Node{}
		yaml.Unmarshal(nodeYAML, n)
		objs = append(objs, n)

		n.Name = fmt.Sprintf("node-%v", i)
		n.Status.Phase = phases[i%len(phases)]
	}

	client := fake.NewSimpleClientset(objs...)

	return client
}

var nodeYAML = []byte(`apiVersion: v1
kind: Node
metadata:
  annotations:
    container.googleapis.com/instance_id: "235558051454394976"
    csi.volume.kubernetes.io/nodeid: '{"filestore.csi.storage.gke.io":"gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf","pd.csi.storage.gke.io":"projects/nais-io/zones/europe-north1-c/instances/gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf"}'
    node.alpha.kubernetes.io/ttl: "0"
    node.gke.io/last-applied-node-labels: addon.gke.io/node-local-dns-ds-ready=true,cloud.google.com/gke-boot-disk=pd-standard,cloud.google.com/gke-container-runtime=containerd,cloud.google.com/gke-netd-ready=true,cloud.google.com/gke-nodepool=nap-1acv0bno,cloud.google.com/gke-os-distribution=cos,cloud.google.com/machine-family=e2,iam.gke.io/gke-metadata-server-enabled=true
    node.gke.io/last-applied-node-taints: ""
    volumes.kubernetes.io/controller-managed-attach-detach: "true"
  creationTimestamp: "2022-03-28T11:32:10Z"
  labels:
    addon.gke.io/node-local-dns-ds-ready: "true"
    beta.kubernetes.io/arch: amd64
    beta.kubernetes.io/instance-type: e2-standard-2
    beta.kubernetes.io/os: linux
    cloud.google.com/gke-boot-disk: pd-standard
    cloud.google.com/gke-container-runtime: containerd
    cloud.google.com/gke-netd-ready: "true"
    cloud.google.com/gke-nodepool: nap-1acv0bno
    cloud.google.com/gke-os-distribution: cos
    cloud.google.com/machine-family: e2
    failure-domain.beta.kubernetes.io/region: europe-north1
    failure-domain.beta.kubernetes.io/zone: europe-north1-c
    iam.gke.io/gke-metadata-server-enabled: "true"
    kubernetes.io/arch: amd64
    kubernetes.io/hostname: gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf
    kubernetes.io/os: linux
    node.kubernetes.io/instance-type: e2-standard-2
    topology.gke.io/zone: europe-north1-c
    topology.kubernetes.io/region: europe-north1
    topology.kubernetes.io/zone: europe-north1-c
  name: gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf
  resourceVersion: "41362359"
  uid: fec44164-6b53-4443-83ac-1a93761a798c
spec:
  podCIDR: 10.0.8.0/26
  podCIDRs:
  - 10.0.8.0/26
  providerID: gce://nais-io/europe-north1-c/gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf
status:
  addresses:
  - address: 10.0.0.29
    type: InternalIP
  - address: gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf.europe-north1-c.c.nais-io.internal
    type: InternalDNS
  - address: gk3-nais-io-nap-1acv0bno-8e1cf148-kkdf.europe-north1-c.c.nais-io.internal
    type: Hostname
  allocatable:
    attachable-volumes-gce-pd: "127"
    cpu: 1930m
    ephemeral-storage: "47093746742"
    hugepages-1Gi: "0"
    hugepages-2Mi: "0"
    memory: 6186972Ki
    pods: "32"
  capacity:
    attachable-volumes-gce-pd: "127"
    cpu: "2"
    ephemeral-storage: 98868448Ki
    hugepages-1Gi: "0"
    hugepages-2Mi: "0"
    memory: 8152028Ki
    pods: "32"
  conditions:
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: kernel has no deadlock
    reason: KernelHasNoDeadlock
    status: "False"
    type: KernelDeadlock
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: Filesystem is not read-only
    reason: FilesystemIsNotReadOnly
    status: "False"
    type: ReadonlyFilesystem
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: docker overlay2 is functioning properly
    reason: NoCorruptDockerOverlay2
    status: "False"
    type: CorruptDockerOverlay2
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: node is functioning properly
    reason: NoFrequentUnregisterNetDevice
    status: "False"
    type: FrequentUnregisterNetDevice
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: kubelet is functioning properly
    reason: NoFrequentKubeletRestart
    status: "False"
    type: FrequentKubeletRestart
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: docker is functioning properly
    reason: NoFrequentDockerRestart
    status: "False"
    type: FrequentDockerRestart
  - lastHeartbeatTime: "2022-05-06T09:39:09Z"
    lastTransitionTime: "2022-03-28T11:32:15Z"
    message: containerd is functioning properly
    reason: NoFrequentContainerdRestart
    status: "False"
    type: FrequentContainerdRestart
  - lastHeartbeatTime: "2022-05-06T01:14:04Z"
    lastTransitionTime: "2022-05-06T01:14:04Z"
    message: NodeController create implicit route
    reason: RouteCreated
    status: "False"
    type: NetworkUnavailable
  - lastHeartbeatTime: "2022-05-06T09:36:51Z"
    lastTransitionTime: "2022-03-28T11:32:08Z"
    message: kubelet has sufficient memory available
    reason: KubeletHasSufficientMemory
    status: "False"
    type: MemoryPressure
  - lastHeartbeatTime: "2022-05-06T09:36:51Z"
    lastTransitionTime: "2022-03-28T11:32:08Z"
    message: kubelet has no disk pressure
    reason: KubeletHasNoDiskPressure
    status: "False"
    type: DiskPressure
  - lastHeartbeatTime: "2022-05-06T09:36:51Z"
    lastTransitionTime: "2022-03-28T11:32:08Z"
    message: kubelet has sufficient PID available
    reason: KubeletHasSufficientPID
    status: "False"
    type: PIDPressure
  - lastHeartbeatTime: "2022-05-06T09:36:51Z"
    lastTransitionTime: "2022-03-28T11:32:30Z"
    message: kubelet is posting ready status. AppArmor enabled
    reason: KubeletReady
    status: "True"
    type: Ready
  daemonEndpoints:
    kubeletEndpoint:
      Port: 10250
  images:
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:4d068218462a755853d9a8a411ebd498bea45fc859e177ad67a619b21162ab52
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-79da2f4
    sizeBytes: 299643715
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:ad4cf980bd1e9d3a64cc8a29cfe4c35db5ac0a4400ab37db71fd502242e03399
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-f440a58
    sizeBytes: 297482291
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:5763188354991b2f6b338b6f12037313e8660c47652b2555d63568a6f8f57f79
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-aacb638
    sizeBytes: 294335469
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:f4faf35fb323d77b83378a03b9de03ab44116e9da1a775a608096bff7baf9715
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-028ef63
    sizeBytes: 292852627
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:b26b1d6bc01f722a8227e9cf4b75daf0e41d921a9b1ed2df855cd0780d561652
    sizeBytes: 292852163
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:7403f585da89369217dc4b27cd031f5cef3963827f887cecfc78598ee8174b74
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:main
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-2df08d8
    sizeBytes: 292844746
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:f4646fe81175ddcf4480a06311c76dfc281fdb72ce901e8b9a6e996aec97dd3a
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-123c116
    sizeBytes: 292831678
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:64d012b09b9f2a0cc3cd686ddf058a4c56cbdd475250b0b9c3132f1191b12150
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-922cc87
    sizeBytes: 237137751
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:e108aaffeb472b913cc22e4beff3e2bc00c842fb5cd681b6a6781091d617f291
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-748114f
    sizeBytes: 236996564
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:43c515fa72767a89039c9226fe899951dc5a6936a154a43c84f42d6aa8ded945
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-86d8190
    sizeBytes: 236935649
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:690cf86d11b89a3479c20cdf62d5fc98a067b595992813139ebec9a9dd6543d5
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-e6e3c4e
    sizeBytes: 216261076
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:c03d7a23a6c664d9f2a801fbc71978a5b8374c1987b10685c034910937fbce64
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-b09491b
    sizeBytes: 216253854
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:d8262db9cc5593eca07da6541911a398ef9bbbb52017f7978b991ecb7e3219b0
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-0830c6d
    sizeBytes: 216217380
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:374ac3d429994544821033511e0f9884e9e0e7a76302c08e92dc637c5be63762
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-b5b840b
    sizeBytes: 216211599
  - names:
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend@sha256:b4b9f373a75f8adefbf0723da6891f071cacf338e228246839715b4d4e88a165
    - europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-frontend:sha-ef68ae9
    sizeBytes: 216196121
  - names:
    - gke.gcr.io/gcp-filestore-csi-driver@sha256:bf03df7f6bafb4bad62437e966e585d1c19b6d2f1969af6bc22967506c54bca9
    - gke.gcr.io/gcp-filestore-csi-driver:v1.1.4-gke.0
    sizeBytes: 112348106
  - names:
    - gke.gcr.io/kube-proxy-amd64:v1.21.9-gke.1002
    - k8s.gcr.io/kube-proxy-amd64:v1.21.9-gke.1002
    sizeBytes: 107470023
  - names:
    - k8s.gcr.io/ingress-nginx/controller@sha256:0bc88eb15f9e7f84e8e56c14fa5735aaa488b840983f87bd79b1054190e660de
    sizeBytes: 103512582
  - names:
    - gke.gcr.io/gcp-filestore-csi-driver@sha256:d4105b0fd758c1228c793240b64f6896463a0a096a68da712b466de156412d6d
    - gke.gcr.io/gcp-filestore-csi-driver:v1.1.3-gke.0
    sizeBytes: 83710366
  - names:
    - gke.gcr.io/gcp-compute-persistent-disk-csi-driver@sha256:9afa38d0d3df00535a09aa89e577b8142b1e7a5d24281655408810e61a8fc4c6
    - gke.gcr.io/gcp-compute-persistent-disk-csi-driver:v1.3.5-gke.0
    sizeBytes: 70075069
  - names:
    - gke.gcr.io/gcp-compute-persistent-disk-csi-driver@sha256:8522f566bd85b0c5cfd8e74a8223ada66bd774c8279b5242903bf796b141b493
    - gke.gcr.io/gcp-compute-persistent-disk-csi-driver:v1.3.6-gke.0
    sizeBytes: 70071363
  - names:
    - gke.gcr.io/k8s-dns-node-cache@sha256:92f84c2670240a2fb7ddf13f4cc9db19094232645225e389bf033f8cbb91a015
    - gke.gcr.io/k8s-dns-node-cache:1.21.1-gke.0
    sizeBytes: 46558448
  - names:
    - gke.gcr.io/k8s-dns-kube-dns@sha256:b5dd662f1a366bbc034954dcc66beb2a5009a78982479f2b7ab7d431b12efb3f
    - gke.gcr.io/k8s-dns-kube-dns:1.21.0-gke.0
    sizeBytes: 43012814
  - names:
    - gke.gcr.io/k8s-dns-sidecar@sha256:6a175b4ddbff9d87551437c481581f7c26444ff678ddf98d16bb458df75e0eb8
    - gke.gcr.io/k8s-dns-sidecar:1.21.0-gke.0
    sizeBytes: 41007040
  - names:
    - gke.gcr.io/k8s-dns-dnsmasq-nanny@sha256:64b131898a7aead50510baa425a0525aa71b2b2733ea0352e50ccdebad682720
    - gke.gcr.io/k8s-dns-dnsmasq-nanny:1.21.0-gke.0
    sizeBytes: 40101539
  nodeInfo:
    architecture: amd64
    bootID: 83e94cae-a54b-497c-9ea8-9a378ebb6977
    containerRuntimeVersion: containerd://1.4.8
    kernelVersion: 5.4.170+
    kubeProxyVersion: v1.21.9-gke.1002
    kubeletVersion: v1.21.9-gke.1002
    machineID: 1dba9c59ffd29e43880b84bc657f0364
    operatingSystem: linux
    osImage: Container-Optimized OS from Google
    systemUUID: 1dba9c59-ffd2-9e43-880b-84bc657f0364
`)
