package model

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/iancoleman/strcase"
)

type KubernetesNode struct {
	Name                    string                     `json:"name"`
	KernelVersion           string                     `json:"kernelVersion"`
	OSImage                 string                     `json:"osImage"`
	ContainerRuntimeVersion string                     `json:"containerRuntimeVersion"`
	KubeletVersion          string                     `json:"kubeletVersion"`
	KubeProxyVersion        string                     `json:"kubeProxyVersion"`
	OperatingSystem         string                     `json:"operatingSystem"`
	Architecture            string                     `json:"architecture"`
	InternalIP              string                     `json:"internalIP"`
	Conditions              []*KubernetesNodeCondition `json:"conditions"`
	Allocatable             *KubernetesNodeResources   `json:"allocatable"`
	Capacity                *KubernetesNodeResources   `json:"capacity"`
}

type KubernetesNodeCondition struct {
	Type           KubernetesNodeConditionType `json:"type"`
	Status         ConditionStatus             `json:"status"`
	Reason         string                      `json:"reason"`
	Message        string                      `json:"message"`
	LastHeartbeat  time.Time                   `json:"lastHeartbeat"`
	LastTransition time.Time                   `json:"lastTransition"`
}

type KubernetesNodeResources struct {
	CPU     int `json:"cpu"`
	Memory  int `json:"memory"`
	Storage int `json:"storage"`
	Pods    int `json:"pods"`
}

type ConditionStatus string

const (
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"
	ConditionStatusUnknown ConditionStatus = "Unknown"
)

var AllConditionStatus = []ConditionStatus{
	ConditionStatusTrue,
	ConditionStatusFalse,
	ConditionStatusUnknown,
}

func (e ConditionStatus) IsValid() bool {
	switch e {
	case ConditionStatusTrue, ConditionStatusFalse, ConditionStatusUnknown:
		return true
	}
	return false
}

func (e ConditionStatus) String() string {
	return string(e)
}

func (e *ConditionStatus) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	str = strcase.ToCamel(str)

	*e = ConditionStatus(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ConditionStatus", str)
	}
	return nil
}

func (e ConditionStatus) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(strcase.ToScreamingSnake(e.String())))
}

type KubernetesNodeConditionType string

const (
	KubernetesNodeConditionTypeReady              KubernetesNodeConditionType = "Ready"
	KubernetesNodeConditionTypeMemoryPressure     KubernetesNodeConditionType = "MemoryPressure"
	KubernetesNodeConditionTypeDiskPressure       KubernetesNodeConditionType = "DiskPressure"
	KubernetesNodeConditionTypePidPressure        KubernetesNodeConditionType = "PIDPressure"
	KubernetesNodeConditionTypeNetworkUnavailable KubernetesNodeConditionType = "NetworkUnavailable"
)

var AllKubernetesNodeConditionType = []KubernetesNodeConditionType{
	KubernetesNodeConditionTypeReady,
	KubernetesNodeConditionTypeMemoryPressure,
	KubernetesNodeConditionTypeDiskPressure,
	KubernetesNodeConditionTypePidPressure,
	KubernetesNodeConditionTypeNetworkUnavailable,
}

func (e KubernetesNodeConditionType) IsValid() bool {
	switch e {
	case KubernetesNodeConditionTypeReady, KubernetesNodeConditionTypeMemoryPressure, KubernetesNodeConditionTypeDiskPressure, KubernetesNodeConditionTypePidPressure, KubernetesNodeConditionTypeNetworkUnavailable:
		return true
	}
	return false
}

func (e KubernetesNodeConditionType) String() string {
	return string(e)
}

func (e *KubernetesNodeConditionType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	str = strcase.ToCamel(str)

	*e = KubernetesNodeConditionType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid KubernetesNodeConditionType", str)
	}
	return nil
}

func (e KubernetesNodeConditionType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(strcase.ToScreamingSnake(e.String())))
}
