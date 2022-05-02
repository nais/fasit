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
