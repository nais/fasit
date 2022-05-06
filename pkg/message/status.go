package message

import (
	"time"

	"github.com/nais/fasit/pkg/graph/model"
)

type Status struct {
	Tenant      string
	Environment string
	Type        StatusType
	Data        []byte
}

type StatusType int

const (
	StatusTypeKubernetesEvent StatusType = iota + 1
	StatusTypeHelm
	StatusTypeHelmReleases
	StatusTypeHealth
	StatusKubernetesNodes
)

type Helm struct {
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version       string
	RolloutStatus model.RolloutStatus
	ConfigHash    string
	Log           string
}

type Health struct {
	Kind       model.EnvironmentKind
	ReportedAt time.Time
}

type Release struct {
	Name         string
	Version      string
	Status       string
	Revision     int
	LastDeployed time.Time
}

type HelmRelease struct {
	Created  time.Time
	Releases []Release
}

type KubernetesNodes struct {
	Nodes []KubernetesNode
}

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type KubernetesNodeConditionType string

const (
	// NodeReady means kubelet is healthy and ready to accept pods.
	NodeReady KubernetesNodeConditionType = "Ready"
	// NodeMemoryPressure means the kubelet is under pressure due to insufficient available memory.
	NodeMemoryPressure KubernetesNodeConditionType = "MemoryPressure"
	// NodeDiskPressure means the kubelet is under pressure due to insufficient available disk.
	NodeDiskPressure KubernetesNodeConditionType = "DiskPressure"
	// NodePIDPressure means the kubelet is under pressure due to insufficient available PID.
	NodePIDPressure KubernetesNodeConditionType = "PIDPressure"
	// NodeNetworkUnavailable means that network for the node is not correctly configured.
	NodeNetworkUnavailable KubernetesNodeConditionType = "NetworkUnavailable"
)

type KubernetesNodeCondition struct {
	Type           KubernetesNodeConditionType
	Status         ConditionStatus
	Reason         string
	Message        string
	LastHeartbeat  time.Time
	LastTransition time.Time
}

type KubernetesNodeResources struct {
	CPU     int64
	Memory  int64
	Storage int64
	Pods    int64
}

type KubernetesNode struct {
	Name                    string
	KernelVersion           string
	OSImage                 string
	ContainerRuntimeVersion string
	KubeletVersion          string
	KubeProxyVersion        string
	OperatingSystem         string
	Architecture            string
	Conditions              []KubernetesNodeCondition
	Allocatable             KubernetesNodeResources
	Capacity                KubernetesNodeResources
}
