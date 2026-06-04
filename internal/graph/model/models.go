package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// EnvironmentCreate contains metadata for creating an environment
type EnvironmentCreate struct {
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	TenantID    uuid.UUID         `json:"tenantID"`
	Kind        EnvironmentKind   `json:"kind"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type EnvironmentLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HelmValueDiff struct {
	Difference HelmValueDifference `json:"difference"`
	Diff       string              `json:"diff"`
}

type RolloutLog struct {
	ID          uuid.UUID  `json:"id"`
	TenantName  string     `json:"tenantName"`
	Environment string     `json:"environment"`
	Lines       []*LogLine `json:"lines"`
}

type TenantCreate struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type UpdateConfiguration struct {
	Description *string         `json:"description,omitempty"`
	Value       json.RawMessage `json:"value"`
}

type ConfigSource string

const (
	ConfigSourceGlobal  ConfigSource = "GLOBAL"
	ConfigSourceEnv     ConfigSource = "ENV"
	ConfigSourceHelm    ConfigSource = "HELM"
	ConfigSourceUnknown ConfigSource = "UNKNOWN"
)

func (e ConfigSource) IsValid() bool {
	switch e {
	case ConfigSourceGlobal, ConfigSourceEnv, ConfigSourceHelm, ConfigSourceUnknown:
		return true
	}
	return false
}

func (e ConfigSource) String() string {
	return string(e)
}

type HelmValueDifference string

const (
	HelmValueDifferenceFullMatch     HelmValueDifference = "FULL_MATCH"
	HelmValueDifferenceSupersetMatch HelmValueDifference = "SUPERSET_MATCH"
	HelmValueDifferenceNoMatch       HelmValueDifference = "NO_MATCH"
	HelmValueDifferenceInvalidJSON   HelmValueDifference = "INVALID_JSON"
)
